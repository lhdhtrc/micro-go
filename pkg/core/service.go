package micro

type Register interface {
	Install(service *ServiceNode) error
	Uninstall()
	SustainLease()
	WithRetryBefore(func())
	WithRetryAfter(func())
	WithLog(func(level LogLevel, message string))
}

type Discovery interface {
	GetService(name string) ([]*ServiceNode, error)
	Watcher()
	Unwatch()
}
