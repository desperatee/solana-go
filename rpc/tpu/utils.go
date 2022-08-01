package tpu

import (
	"strconv"
	"strings"
)

func CheckIfDuplicate(array []string, item string) bool {
	for _, value := range array {
		if value == item {
			return true
		}
	}
	return false
}

func ConvertURLToWS(url string) string {
	urlWithReplacedProtocol := strings.Replace(strings.Replace(url, "https", "wss", -1), "http", "ws", -1)
	protocolSplit := strings.Split(urlWithReplacedProtocol, "://")
	if strings.Contains(protocolSplit[1], ":") {
		var port string = ""
		portSplit := strings.Split(protocolSplit[1], ":")
		if len(portSplit) == 2 {
			portInt, err := strconv.Atoi(strings.Replace(portSplit[1], "/", "", -1))
			if err != nil {
				return ""
			}
			port = strconv.Itoa(portInt + 1)
			return protocolSplit[0] + "://" + portSplit[0] + ":" + port
		} else {
			return ""
		}
	} else {
		return urlWithReplacedProtocol
	}
}
