# envoy-management

management place for envoy

This is the the management server, that control configuration for envoy nodes.
It's uses go-control-plane for communication with envoy.
There are 3 major components:
main.go - ADS server. Uses mongodb as backend.
ratelimit.go - global ratelimit service, inspired by implementation from Lyft. Uses mongodb for configuration, redis for key/value storage
als.go - access log collector. Uses elasticsearch for remote logging
ipmgmt_server.go - client-side daemon, that can be used for IP address and routing manipulation at envoy node side. Uses NetworkManager (nmcli)

Configuration for all of this components handled by main server. It implements northbound REST API, which can be used for config manipulation.
Swagger available at <node-ip>:8080/swagger/index.html
Also there is separate UI project "envoy-frontend", that utilize this api.
