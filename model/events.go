package model

type HostAdded struct {
	Host Host
}

type HostDeleted struct {
	HostKey HostKey
}

type KeyPairUpdated struct {
	Host    Host
	KeyPair KeyPair
}

type ListenerAdded struct {
	Host     Host
	Listener Listener
}

type ListenerDeleted struct {
	Listener ListenerKey
}

type FrontendAdded struct {
	Frontend Frontend
}

type FrontendDeleted struct {
	Frontend FrontendKey
}

type FrontendBackendUpdated struct {
	Frontend Frontend
	Backend  Backend
}

type FrontendRouteUpdated struct {
	Frontend Frontend
}

type FrontendOptionsUpdated struct {
	Frontend Frontend
}

type MiddlewareAdded struct {
	Frontend   Frontend
	Middleware Middleware
}

type MiddlewareUpdated struct {
	Frontend   Frontend
	Middleware Middleware
}

type MiddlewareDeleted struct {
	Frontend     Frontend
	MidlewareKey MiddlewareKey
}

type BackendAdded struct {
	Backend Backend
}

type BackendDeleted struct {
	BackendKey BackendKey
}

type BackendOptionsUpdated struct {
	Backend Backend
}

type ServerAdded struct {
	Backend Backend
	Server  Server
}

type ServerUpdated struct {
	Backend Backend
	Server  Server
}

type ServerDeleted struct {
	Backend   Backend
	ServerKey ServerKey
}
