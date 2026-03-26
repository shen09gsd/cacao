package api

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lanthora/cacao/candy"
	"github.com/lanthora/cacao/model"
	"github.com/lanthora/cacao/storage"
)

func notIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip == nil || ip.To4() == nil
}

func RouteShow(c *gin.Context) {
	user := c.MustGet("user").(*model.User)
	routes := model.GetRoutesByUserID(user.ID)

	type routeinfo struct {
		RouteID  uint   `json:"routeid"`
		NetID    uint   `json:"netid"`
		DevAddr  string `json:"devaddr"`
		DevMask  string `json:"devmask"`
		DstAddr  string `json:"dstaddr"`
		DstMask  string `json:"dstmask"`
		NextHop  string `json:"nexthop"`
		Priority int    `json:"priority"`
	}

	response := make([]routeinfo, 0)
	for _, r := range routes {
		response = append(response, routeinfo{
			RouteID:  r.ID,
			NetID:    r.NetID,
			DevAddr:  r.DevAddr,
			DevMask:  r.DevMask,
			DstAddr:  r.DstAddr,
			DstMask:  r.DstMask,
			NextHop:  r.NextHop,
			Priority: r.Priority,
		})
	}

	setResponseData(c, gin.H{
		"routes": response,
	})
}

func RouteInsert(c *gin.Context) {
	var request struct {
		NetID    uint   `json:"netid"`
		DevAddr  string `json:"devaddr"`
		DevMask  string `json:"devmask"`
		DstAddr  string `json:"dstaddr"`
		DstMask  string `json:"dstmask"`
		NextHop  string `json:"nexthop"`
		Priority int    `json:"priority"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		setErrorCode(c, InvalidRequest)
		return
	}

	user := c.MustGet("user").(*model.User)
	netModel := model.GetNetByNetID(request.NetID)
	if netModel.UserID != user.ID {
		setErrorCode(c, RouteNotExists)
		return
	}

	if notIPv4(request.DevAddr) || notIPv4(request.DevMask) || notIPv4(request.DstAddr) || notIPv4(request.DstMask) || notIPv4(request.NextHop) {
		setErrorCode(c, InvalidIPAddress)
		return
	}

	routeModel := model.Route{
		NetID:    request.NetID,
		DevAddr:  request.DevAddr,
		DevMask:  request.DevMask,
		DstAddr:  request.DstAddr,
		DstMask:  request.DstMask,
		NextHop:  request.NextHop,
		Priority: request.Priority,
	}
	routeModel.Create()
	candy.ReloadNet(netModel.ID)

	setResponseData(c, gin.H{
		"routeid":  routeModel.ID,
		"netid":    routeModel.NetID,
		"devaddr":  routeModel.DevAddr,
		"devmask":  routeModel.DevMask,
		"dstaddr":  routeModel.DstAddr,
		"dstmask":  routeModel.DstMask,
		"nexthop":  routeModel.NextHop,
		"priority": routeModel.Priority,
	})
}

func cidrToMask(cidr string) (string, error) {
	cidrInt := 0
	for _, c := range cidr {
		if c == '/' {
			break
		}
		if c >= '0' && c <= '9' {
			cidrInt = cidrInt*10 + int(c-'0')
		}
	}
	if cidrInt < 0 || cidrInt > 32 {
		return "", fmt.Errorf("invalid CIDR prefix: %d", cidrInt)
	}
	if cidrInt == 0 {
		return "0.0.0.0", nil
	}
	mask := ^(uint32(0)) << (32 - cidrInt)
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(mask>>24), byte(mask>>16), byte(mask>>8), byte(mask)), nil
}

func RouteImport(c *gin.Context) {
	netIDStr := c.PostForm("netid")
	netID, err := strconv.ParseUint(netIDStr, 10, 64)
	if err != nil {
		setErrorCode(c, InvalidRequest)
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		setErrorCode(c, InvalidRequest)
		return
	}

	f, err := file.Open()
	if err != nil {
		setErrorCode(c, InvalidRequest)
		return
	}
	defer f.Close()

	buf := make([]byte, file.Size)
	if _, err := f.Read(buf); err != nil {
		setErrorCode(c, InvalidRequest)
		return
	}

	user := c.MustGet("user").(*model.User)
	netModel := model.GetNetByNetID(uint(netID))
	if netModel.UserID != user.ID {
		setErrorCode(c, RouteNotExists)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	var routes []model.Route
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			setErrorCode(c, InvalidRequest)
			return
		}

		devCIDR := strings.TrimSpace(parts[0])
		dstCIDR := strings.TrimSpace(parts[1])
		nextHop := strings.TrimSpace(parts[2])

		devAddr, devCIDRSuffix, found := strings.Cut(devCIDR, "/")
		if !found {
			setErrorCode(c, InvalidRequest)
			return
		}
		dstAddr, dstCIDRSuffix, found := strings.Cut(dstCIDR, "/")
		if !found {
			setErrorCode(c, InvalidRequest)
			return
		}

		if notIPv4(devAddr) || notIPv4(dstAddr) || notIPv4(nextHop) {
			setErrorCode(c, InvalidIPAddress)
			return
		}

		devMask, err := cidrToMask(devCIDRSuffix)
		if err != nil {
			setErrorCode(c, InvalidIPAddress)
			return
		}
		dstMask, err := cidrToMask(dstCIDRSuffix)
		if err != nil {
			setErrorCode(c, InvalidIPAddress)
			return
		}

		if notIPv4(devMask) || notIPv4(dstMask) {
			setErrorCode(c, InvalidIPAddress)
			return
		}

		routes = append(routes, model.Route{
			NetID:    uint(netID),
			DevAddr:  devAddr,
			DevMask:  devMask,
			DstAddr:  dstAddr,
			DstMask:  dstMask,
			NextHop:  nextHop,
			Priority: 0,
		})
	}

	if len(routes) > 0 {
		db := storage.Get()
		tx := db.Begin()
		for i := range routes {
			if err := tx.Create(&routes[i]).Error; err != nil {
				tx.Rollback()
				setUnexpectedMessage(c, err.Error())
				return
			}
		}
		tx.Commit()
		candy.ReloadNet(netModel.ID)
	}

	setResponseData(c, gin.H{
		"count": len(routes),
	})
}

func RouteDelete(c *gin.Context) {
	var request struct {
		RouteID uint `json:"routeid"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		setErrorCode(c, InvalidRequest)
		return
	}

	routeModel := model.GetRouteByRouteID(request.RouteID)
	netModel := model.GetNetByNetID(routeModel.NetID)

	user := c.MustGet("user").(*model.User)
	if user.ID != netModel.UserID {
		setErrorCode(c, RouteNotExists)
		return
	}

	routeModel.Delete()
	candy.ReloadNet(netModel.ID)
	setResponseData(c, nil)
}
