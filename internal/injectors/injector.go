package injectors

type Injector interface {
	Inject() (map[string]string, error)

	String() string
}
