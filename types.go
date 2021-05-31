package godepi

type (
	ProviderName struct {
		Name    string
		Factory FactoryFunc
	}

	ProviderInterface struct {
		Provide interface{}
		Factory FactoryFunc
	}

	FactoryFunc   interface{}
)

const (
	INJECT = "inject"
	NESTED = "nested"
)
