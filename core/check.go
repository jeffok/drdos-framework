package core

import (
	"drdos/config"
	"drdos/core/drdos"
	"drdos/utils"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

// 修改一下，放大倍数统计应该是包的大小累加

var (
	ipch   chan map[string]int
	checks map[string]interface{}
)

func init() {
	ipch = make(chan map[string]int, 10000) // 管道大小(应该可以吧)
	checks = map[string]interface{}{
		"ntp":     drdos.CheckNtp,
		"dns":     drdos.CheckDns,
		"ssdp":    drdos.CheckSsdp,
		"snmp":    drdos.CheckSnmp,
		"mem":     drdos.CheckMemcache,
		"portmap": drdos.CheckPortmap,
		"ldap":    drdos.CheckLdap,
	}
}

// Check drdos ip check
func Check(iplist []string, atktype string, outputfile string, interval uint, publicip string) (map[string]int, error) {
	// 变量初始化
	result := make(map[string]int)
	dir, _ := os.Getwd()

	// 校验是否有公网IP
	if publicip == "" {
		tempip, err := utils.PublicIP()
		publicip = tempip
		if err != nil {
			fmt.Println("[!] Public ipaddr not found!")
			return result, err
		}
	}
	fmt.Println("[+] PublicIP is :", publicip)
	fmt.Println("[+] Start Checking iplist")

	// 开始监听
	udpaddr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:"+strconv.Itoa(config.ListenPort))
	if err != nil {
		log.Fatal("[!] Listen err: [%v]", err)
	}
	udpconn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		log.Fatal("[!] Listen err: [%v]", err)
	}

	// 匿名函数生产
	go func() {
		for {
			clientHandle(udpconn)
		}
	}()

	// 匿名函数消费
	go func() {
		for {
			select {
			case data := <-ipch:
				for index, value := range data {
					if _, ok := result[index]; ok {
						result[index] = result[index] + value
					} else {
						if utils.IsContain(iplist, index) {
							result[index] = value
							err := utils.FileWrites(dir+"/data/results/"+outputfile, index)
							if err != nil {
								return
							}
						}
					}
				}
			}
		}
	}()

	// 这里开始循环遍历IP发送
	for index, ipaddr := range iplist {
		time.Sleep(time.Duration(interval) * time.Microsecond)
		if _, ok := checks[atktype]; ok {
			utils.Call(checks, atktype, ipaddr, publicip)
			utils.ProcessBar(index+1, len(iplist))
		} else {
			fmt.Println("[!] Atktype not found")
			err := errors.New("[!] Atktype not found")
			return result, err
		}
	}
	// 等待，接收剩余包
	time.Sleep(config.WaitTime * time.Second)
	fmt.Println("[+] Finished, Total count : " + strconv.Itoa(len(result)))
	fmt.Println("[+] Result path : " + dir + "/data/results/" + outputfile)
	return result, nil
}

// clientHandle 函数中用包的大小来做判断其实挺不靠谱的，比较好的解决方法是对返回包做个回值校验
func clientHandle(conn *net.UDPConn) {
	var buf [1024]byte
	length, addr, err := conn.ReadFromUDP(buf[:])
	if err != nil {
		return
	}
	if length > config.Threshold {
		ipch <- map[string]int{addr.IP.String(): length}
	}
	return
}
