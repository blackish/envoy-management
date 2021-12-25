package ipmgmt

type NodeInterfaceConfigType struct {
	Name  string   `json:"name"`
	Ipnet []string `json:"ipnet"`
}

type NodeInterfaceListConfigType struct {
	Ifaces []NodeInterfaceConfigType `json:"ifaces"`
}

type NodeRouteConfigType struct {
	Net     string `json:"net"`
	Iface   string `json:"iface"`
	Nexthop string `json:"nexthop"`
	Src     string `json:"src"`
}

type NodeRouteListConfigType struct {
	Routes []NodeRouteConfigType `json:"routes"`
}
