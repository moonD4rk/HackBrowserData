package injector

type Strategy interface {
	Inject(exePath string, payload []byte, env map[string]string) ([]byte, error)
}
