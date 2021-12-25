package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"ipmgmt"
	"net"
	"os/exec"
	"strings"
)

func getLocalInterfaces() (*ipmgmt.NodeInterfaceListConfigType, error) {
	ifaceList := &ipmgmt.NodeInterfaceListConfigType{}
	ifaceList.Ifaces = make([]ipmgmt.NodeInterfaceConfigType, 0)
	cmd := exec.Command("/usr/bin/nmcli", "--mode", "multiline", "dev", "show")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ifaceList, err
	}
	for line, err := out.ReadString('\n'); err == nil; line, err = out.ReadString('\n') {
		keyValue := strings.Split(line, ":")
		for k := 0; k < len(keyValue); k += 2 {
			if keyValue[k] == "GENERAL.DEVICE" {
				ifaceList.Ifaces = append(ifaceList.Ifaces, ipmgmt.NodeInterfaceConfigType{Name: strings.TrimSpace(keyValue[k+1])})
			}
		}
	}
	for ind, v := range ifaceList.Ifaces {
		cmd = exec.Command("/usr/bin/nmcli", "--mode", "multiline", "--fields", "IP4.ADDRESS", "dev", "show", v.Name)
		v.Ipnet = make([]string, 0)
		cmd.Stdout = &out
		err = cmd.Run()
		if err == nil {
			for line, err := out.ReadString('\n'); err == nil; line, err = out.ReadString('\n') {
				kv := strings.Split(line, ":")
				ifaceList.Ifaces[ind].Ipnet = append(ifaceList.Ifaces[ind].Ipnet, strings.TrimSpace(kv[1]))
			}
		}
	}
	return ifaceList, nil
}
func GetLocalInterfaces(ctx *gin.Context) {
	ifaceList, err := getLocalInterfaces()
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to get interfaces"})
		return
	}
	body, _ := json.Marshal(ifaceList)
	ctx.Data(200, "application/json", body)
	return
}

func GetLocalRoutes(ctx *gin.Context) {
	ifaceList, err := getLocalInterfaces()
	routeList := &ipmgmt.NodeRouteListConfigType{}
	routeList.Routes = make([]ipmgmt.NodeRouteConfigType, 0)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to get routes"})
		return
	}
	var out bytes.Buffer
	for _, iface := range ifaceList.Ifaces {
		cmd := exec.Command("/usr/bin/nmcli", "--mode", "multiline", "--fields", "IP4.ROUTE", "conn", "show", iface.Name)
		cmd.Stdout = &out
		if err = cmd.Run(); err == nil {
			for line, err := out.ReadString('\n'); err == nil; line, err = out.ReadString('\n') {
				kv := strings.Split(line, ":")
				lroute := strings.Split(strings.TrimSpace(kv[1]), " ")
				routeList.Routes = append(routeList.Routes, ipmgmt.NodeRouteConfigType{Net: strings.TrimRight(lroute[2], ","), Iface: iface.Name, Nexthop: strings.TrimRight(lroute[5], ",")})
			}
		}
	}
	body, _ := json.Marshal(routeList)
	ctx.Data(200, "application/json", body)
	return
}

func SetLocalInterface(ctx *gin.Context) {
	nodeInterface := &ipmgmt.NodeInterfaceConfigType{}
	if err := ctx.ShouldBindJSON(&nodeInterface); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	for _, addr := range nodeInterface.Ipnet {
		_, _, err := net.ParseCIDR(addr)
		conName := ""
		if err == nil {
			cmd := exec.Command("/usr/bin/nmcli", "--mode", "multiline", "dev", "show", nodeInterface.Name)
			var out bytes.Buffer
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				ctx.JSON(400, gin.H{"error": "failed to set routes"})
				return
			}
			for line, err := out.ReadString('\n'); err == nil; line, err = out.ReadString('\n') {
				keyValue := strings.Split(line, ":")
				for k := 0; k < len(keyValue); k += 2 {
					if keyValue[k] == "GENERAL.CONNECTION" {
						conName = strings.TrimSpace(keyValue[k+1])
					}
				}
			}
			if conName == "--" {
				cmd := exec.Command("/usr/bin/nmcli", "conn", "add", "con-name", nodeInterface.Name, "ifname", nodeInterface.Name, "type", "ethernet", "ip4", addr)
				err = cmd.Run()
			} else {
				cmd := exec.Command("/usr/bin/nmcli", "conn", "modify", nodeInterface.Name, "+ipv4.address", addr)
				err = cmd.Run()
			}

			if err != nil {
				ctx.JSON(400, gin.H{"error": "failed to set routes"})
				return
			}
		}
	}
	cmd := exec.Command("/usr/bin/nmcli", "conn", "up", nodeInterface.Name)
	_ = cmd.Run()
	ctx.Status(204)
	return
}

func DelLocalInterface(ctx *gin.Context) {
	nodeInterface := &ipmgmt.NodeInterfaceConfigType{}
	if err := ctx.ShouldBindJSON(&nodeInterface); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	for _, addr := range nodeInterface.Ipnet {
		_, _, err := net.ParseCIDR(addr)
		if err == nil {
			cmd := exec.Command("/usr/bin/nmcli", "conn", "modify", nodeInterface.Name, "-ipv4.address", addr)
			err := cmd.Run()
			if err != nil {
				ctx.JSON(400, gin.H{"error": "failed to set routes"})
				return
			}
		}
	}
	cmd := exec.Command("/usr/bin/nmcli", "conn", "up", nodeInterface.Name)
	_ = cmd.Run()
	ctx.Status(204)
	return
}

func SetLocalRoute(ctx *gin.Context) {
	nodeRoute := ipmgmt.NodeRouteConfigType{}
	if err := ctx.ShouldBindJSON(&nodeRoute); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	_, _, err := net.ParseCIDR(nodeRoute.Net)
	if err != nil {
		log.Info(err)
		ctx.JSON(400, gin.H{"error": "failed to parse routes"})
		return
	}
	cmd := exec.Command("/usr/bin/nmcli", "conn", "modify", nodeRoute.Iface, "+ipv4.routes", nodeRoute.Net+" "+nodeRoute.Nexthop)
	err = cmd.Run()
	if err != nil {
		log.Info(err)
		ctx.JSON(400, gin.H{"error": "failed to set routes"})
		return
	}
	cmd = exec.Command("/usr/bin/nmcli", "conn", "up", nodeRoute.Iface)
	err = cmd.Run()
	ctx.Status(204)
	return
}
func DelLocalRoute(ctx *gin.Context) {
	nodeRoute := ipmgmt.NodeRouteConfigType{}
	if err := ctx.ShouldBindJSON(&nodeRoute); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	_, _, err := net.ParseCIDR(nodeRoute.Net)
	if err != nil {
		log.Info(err)
		ctx.JSON(400, gin.H{"error": "failed to parse routes"})
		return
	}
	cmd := exec.Command("/usr/bin/nmcli", "conn", "modify", nodeRoute.Iface, "-ipv4.routes", nodeRoute.Net+" "+nodeRoute.Nexthop)
	err = cmd.Run()
	if err != nil {
		log.Info(err)
		ctx.JSON(400, gin.H{"error": "failed to set routes"})
		return
	}
	cmd = exec.Command("/usr/bin/nmcli", "conn", "up", nodeRoute.Iface)
	err = cmd.Run()
	ctx.Status(204)
	return
}

func main() {
	r := gin.Default()
	v1 := r.Group("/api/v1")
	{
		v1.GET("/interface", GetLocalInterfaces)
		v1.PUT("/interface", SetLocalInterface)
		v1.DELETE("/interface", DelLocalInterface)
		v1.GET("/route", GetLocalRoutes)
		v1.PUT("/route", SetLocalRoute)
		v1.DELETE("/route", DelLocalRoute)
	}
	r.Run(":9902")
	return
}
