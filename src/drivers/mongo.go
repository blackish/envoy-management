package drivers

import (
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

var Cli *mongo.Client
var ConnectTimeout time.Duration
