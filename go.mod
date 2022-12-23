module main

go 1.13

require (
	accesslogs v0.0.0
	authz v0.0.0
	authzs3 v0.0.0
	docs v0.0.0
	drivers v0.0.0
	github.com/ClickHouse/clickhouse-go/v2 v2.2.0 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/coocood/freecache v1.1.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/envoyproxy/go-control-plane v0.10.3
	github.com/gin-gonic/gin v1.6.3
	github.com/go-openapi/jsonreference v0.19.6 // indirect
	github.com/go-openapi/spec v0.20.3 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mediocregopher/radix/v3 v3.6.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/swaggo/files v0.0.0-20190704085106-630677cd5c14
	github.com/swaggo/gin-swagger v1.2.0
	github.com/swaggo/swag v1.7.0 // indirect
	github.com/urfave/cli v1.22.5 // indirect
	go.mongodb.org/mongo-driver v1.4.4
	google.golang.org/grpc v1.45.0
	ipmgmt v0.0.0
	loader v0.0.0
	rapi v0.0.0
	ratelimit v0.0.0
	types v0.0.0
)

replace loader => ./src/loader

replace accesslogs => ./src/accesslogs

replace rapi => ./src/rapi

replace types => ./src/types

replace docs => ./src/docs

replace ipmgmt => ./src/ipmgmt

replace drivers => ./src/drivers

replace ratelimit => ./src/ratelimit

replace authz => ./src/authz

replace authzs3 => ./src/authzs3
