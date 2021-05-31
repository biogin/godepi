package godepi

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

type Container struct {
	imports []*Container

	factories     map[string]FactoryFunc
	instances     map[string]interface{}

	timeout time.Duration
	running bool
}

type ContainerOpts struct {
	Imports []*Container

	ByName        []ProviderName
	ByInterface   []ProviderInterface
	ByFactories   []FactoryFunc
}


func NewContainer(opts *ContainerOpts) *Container {
	c := &Container{
		factories: make(map[string]FactoryFunc),
		instances: make(map[string]interface{}),
		timeout:   10 * time.Second,
	}

	if opts != nil {
		if len(opts.Imports) > 0 {
			for _, importedContainer := range opts.Imports {
				if importedContainer == nil {
					panic("imported container has nil value")
				}

				c.imports = append(c.imports, importedContainer)
			}
		}

		for _, p := range opts.ByName {
			c.Provide(p.Name, p.Factory)
		}
		for _, p := range opts.ByInterface {
			c.Provide(p.Provide, p.Factory)
		}
		for _, f := range opts.ByFactories {
			c.Provide(getFactoryReturnDepName(f), f)
		}
	}

	return c
}

func (c *Container) Run() {
	if c.running {
		panic("container is already initialized")
	}

	c.running = true
}

func (c *Container) ProvideByName(depName string, factory FactoryFunc) {
	c.Provide(depName, factory)
}

func (c *Container) ProvideByInterface(dep interface{}, factory FactoryFunc) {
	c.Provide(dep, factory)
}

func (c *Container) ProvideByFactory(factory FactoryFunc) {
	c.Provide(getFactoryReturnDepName(factory), factory)
}

func (c *Container) Provide(dep interface{}, factory FactoryFunc) {
	if c.running {
		panic("cannot provide new factory in a running container")
	}

	checkFactory(dep, factory)

	depName := getDepName(dep)

	if c.factories[depName] != nil {
		panic(fmt.Sprintf(`"%s" already provided`, depName))
	}
	c.factories[depName] = factory
}

func (c *Container) get(dep interface{}) interface{} {
	depName := getDepName(dep)

	instance := c.GetInstance(depName)
	if instance == nil {
		f := c.GetFactory(depName)
		if f == nil {
			panic(fmt.Sprintf(`"%s" dependency not provided`, depName))
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()

		fCallRes := c.callFunc(ctx, f, depName)

		instance = fCallRes[0].Interface()
		err := c.SetInstance(depName, instance)
		if err != nil {
			panic(err)
		}
		c.injectFields(fCallRes[0])
	}

	return instance
}

func (c *Container) injectFields(v reflect.Value) {
	isInterface := v.Kind() == reflect.Interface
	isPtrToStruct := v.Kind() == reflect.Ptr && v.Elem().Kind() == reflect.Struct
	if v.Kind() == reflect.Struct || isPtrToStruct || isInterface {
		if isPtrToStruct {
			v = v.Elem()
		} else if isInterface {
			v = v.Elem().Elem()
		}

		for i := 0; i < v.Type().NumField(); i++ {
			injectTag := v.Type().Field(i).Tag.Get("di")
			if injectTag == INJECT {
				dep := c.get(getFullDepPath(v.Type().Field(i).Type))
				v.Field(i).Set(reflect.ValueOf(dep))
			}
			if injectTag == NESTED {
				c.injectFields(v.Field(i))
			}
		}
	}
}

func (c *Container) GetInstance(depName string) interface{} {
	instance := c.instances[depName]
	if instance != nil {
		return instance
	} else if f := c.factories[depName]; f != nil {
		// has own factory
		return nil
	}

	if c.imports != nil {
		for _, importedContainer := range c.imports {
			instance = importedContainer.GetInstance(depName)
			if instance != nil {
				return instance
			}
		}
	}

	return nil
}

func (c *Container) SetInstance(depName string, instance interface{}) error {
	if f := c.factories[depName]; f != nil {
		c.instances[depName] = instance
		return nil
	}

	if c.imports != nil {
		for _, importedContainer := range c.imports {
			err := importedContainer.SetInstance(depName, instance)
			if err == nil {
				return nil
			}
		}
	}

	return errors.New("failed to set instance")
}

func (c *Container) GetFactory(depName string) FactoryFunc {
	if f := c.factories[depName]; f != nil {
		return f
	}

	if c.imports != nil {
		for _, importedContainer := range c.imports {
			f := importedContainer.GetFactory(depName)
			if f != nil {
				return f
			}
		}
	}

	return nil
}

func (c *Container) callFunc(ctx context.Context, f interface{}, depName string) []reflect.Value {
	ok := make(chan struct{})

	go func() {
		select {
		case <-ok:
		case <-ctx.Done():
			if ctx.Err() != context.Canceled {
				panic(depName + " call: " + ctx.Err().Error())
			}
		}
	}()

	fType := reflect.TypeOf(f)
	fDepsVals := make([]reflect.Value, fType.NumIn())
	for i := 0; i < fType.NumIn(); i++ {
		fDepType := fType.In(i)
		fDep := c.get(getFullDepPath(fDepType))
		fDepsVals[i] = reflect.ValueOf(fDep)
	}
	fVal := reflect.ValueOf(f)

	fCallRes := fVal.Call(fDepsVals)

	for _, resV := range fCallRes {
		if resV.Type().String() == "error" && resV.Interface() != nil {
			panic(fmt.Sprintf(`"%s" returned error: "%s"`, depName, resV.Interface()))
		}
	}

	ok <- struct{}{}

	return fCallRes
}
