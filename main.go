package godepi

func main() {
	opts := &ContainerOpts{
		Imports:        nil,
		ByName:         []ProviderName{},
		ByInterface:    []ProviderInterface{},
		ByFactories:    nil,
	}
	container := NewContainer(opts)

	container.Run()
}
