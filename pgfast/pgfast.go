package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type vmessConfig struct {
	Address string            `json:"address"`
	Port    int64             `json:"port"`
	Users   []vmessConfigUser `json:"users"`
}
type vmessConfigUser struct {
	Id       string `json:"id"`
	alterId  int64  `json:"alterId"`
	Level    int64  `json:"level"`
	Security string `json:"security"`
}

var default_config = []byte(`
{
  "log": {
    "loglevel": "warning"
  },
  "dns": {
    "servers": [
      {
        "address": "https://1.1.1.1/dns-query",
        "domains": ["geosite:geolocation-!cn", "geosite:google@cn"]
      },
      "8.8.8.8",
      {
        "address": "114.114.114.114",
        "port": 53,
        "domains": [
          "geosite:cn",
          "geosite:icloud",
          "geosite:category-games@cn"
        ],
        "expectIPs": ["geoip:cn"],
        "skipFallback": true
      },
      {
        "address": "localhost",
        "skipFallback": true
      }
    ]
  },
  "inbounds": [
    {
      "protocol": "socks",
      "listen": "0.0.0.0",
      "port": 1080,
      "tag": "Socks-In",
      "settings": {
        "ip": "127.0.0.1",
        "udp": true,
        "auth": "noauth"
      },
      "sniffing": {
        "enabled": true,
        "destOverride": ["http", "tls"]
      }
    },
    {
      "protocol": "http",
      "listen": "0.0.0.0",
      "port": 2080,
      "tag": "Http-In",
      "sniffing": {
        "enabled": true,
        "destOverride": ["http", "tls"]
      }
    }
  ],
  "outbounds": [
    {
      "mux": {
        "concurrency": 8,
        "enabled": true
      },
      "protocol": "vmess",
      "settings": {
        "vnext": []
      },
      "streamSettings": {
        "network": "ws",
        "security": "tls",
        "tlsSettings": {
          "allowInsecure": true
        },
        "wsSettings": {
          "headers": {
            "host": ""
          },
          "path": "/pgf"
        }
      },
      "tag": "proxy"
    },
    {
      "protocol": "dns",
      "tag": "Dns-Out"
    },
    {
      "protocol": "freedom",
      "tag": "Direct",
      "settings": {
        "domainStrategy": "UseIPv4"
      }
    },
    {
      "protocol": "blackhole",
      "tag": "Reject",
      "settings": {
        "response": {
          "type": "http"
        }
      }
    }
  ],
  "routing": {
    "domainStrategy": "IPIfNonMatch",
    "domainMatcher": "mph",
    "rules": [
      {
        "type": "field",
        "outboundTag": "Direct",
        "protocol": ["bittorrent"]
      },
      {
        "type": "field",
        "outboundTag": "Dns-Out",
        "inboundTag": ["Socks-In", "Http-In"],
        "network": "udp",
        "port": 53
      },
      {
        "type": "field",
        "outboundTag": "Reject",
        "domain": ["geosite:category-ads-all"]
      },
      {
        "type": "field",
        "outboundTag": "Proxy",
        "domain": [
          "full:www.icloud.com",
          "domain:icloud-content.com",
          "geosite:google"
        ]
      },
      {
        "type": "field",
        "outboundTag": "Direct",
        "domain": [
          "geosite:tld-cn",
          "geosite:icloud",
          "geosite:category-games@cn"
        ]
      },
      {
        "type": "field",
        "outboundTag": "Proxy",
        "domain": ["geosite:geolocation-!cn"]
      },
      {
        "type": "field",
        "outboundTag": "Direct",
        "domain": ["geosite:cn", "geosite:private"]
      },
      {
        "type": "field",
        "outboundTag": "Direct",
        "ip": ["geoip:cn", "geoip:private"]
      },
      {
        "type": "field",
        "outboundTag": "Proxy",
        "network": "tcp,udp"
      }
    ]
  }
}`)

func main() {
	viper.BindEnv("pgfast_subscribe_url")
	viper.BindEnv("interval")

	subUrl := viper.GetString("pgfast_subscribe_url")
	interval := viper.GetInt32("interval")

	if len(subUrl) == 0 {
		log.Fatalf("SubScribe Url not defined")
	}

	vmessConfig := getVmessConfigFromSubscribe(subUrl)
	generatePgfastConfig(vmessConfig)

	//minitoring config file change and auto restart v2ray
	viper.SetConfigFile("./v2ray-config.json")
	viper.ReadInConfig()
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		cmd := exec.Command("supervisorctl", "restart", "v2ray")
		data, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("failed to call CombinedOutput(): %v", err)
		}
		log.Printf("output: %s", data)
	})

	//update
	ticker := time.NewTicker(time.Minute * time.Duration(interval))
	for _ = range ticker.C {
		vmessConfig := getVmessConfigFromSubscribe(subUrl)
		generatePgfastConfig(vmessConfig)
	}
}

func generatePgfastConfig(vmessConfig []vmessConfig) {
	viper.SetConfigFile("./v2ray-config.json")
	configBuffer, _ := json.Marshal(vmessConfig)
	processConfig, _ := jsonparser.Set(default_config, configBuffer, "outbounds", "[0]", "settings", "vnext")
	//fmt.Println(string(processConfig))
	viper.ReadConfig(bytes.NewBuffer(processConfig))
	viper.WriteConfig()

	t := time.Now()
	fmt.Println(t.Format("2006-01-02 15:04:05"), ": generate config file...")
}
func getVmessConfigFromSubscribe(link string) []vmessConfig {
	resp, err := http.Get(link)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Panic(err) //请求错误, 不应该继续向下执行, panic掉
	}
	defer resp.Body.Close()
	//将数据放入一个 bytes.Buffer 中, 方便操作
	resBuffer := new(bytes.Buffer)
	_, _ = resBuffer.ReadFrom(base64.NewDecoder(base64.RawURLEncoding, resp.Body))
	var result = []vmessConfig{}
	lines := strings.Split(resBuffer.String(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "vmess://") {
			vmessConfigByte, _ := base64.StdEncoding.DecodeString(line[8:])
			vmessConfigString := string(vmessConfigByte)
			//fmt.Println(vmessConfigString)

			ConfigUser := vmessConfigUser{
				alterId:  gjson.Get(vmessConfigString, "aid").Int(),
				Id:       gjson.Get(vmessConfigString, "id").String(),
				Level:    0,
				Security: "aes-128-gcm",
			}
			var users = []vmessConfigUser{}
			users = append(users, ConfigUser)

			config := vmessConfig{
				Address: gjson.Get(vmessConfigString, "add").String(),
				Port:    gjson.Get(vmessConfigString, "port").Int(),
				Users:   users,
			}
			result = append(result, config)
		}
	}
	//result_marshal, _ := json.MarshalIndent(result, "", "  ")
	//fmt.Println(string(result_marshal))
	return result
}