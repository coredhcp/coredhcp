package rangeplugin

type KnownDevice struct {
	Name string   `yaml:"name"`
	Mac  string   `yaml:"mac"`
	IP   string   `yaml:"ip"`
	Tags []string `yaml:"tags"`
}

type KnownDevices struct {
	KnownDevices []KnownDevice `yaml:"known_devices"`
}
