# dns proxy
A simple dns proxy server with in memory cache.

# 功能说明：  
1、支持并发向多台远程 DNS 服务器查询来解决 DNS 稳定性问题  
2、支持基于内存的查询结果缓存来提高查询性能，并支持调整缓存时间  
3、支持不同的域名转发到不同的后端服务器组，满足特定的业务场景需求  
4、支持域名与查询结果映射，可以用于内部非公开的域名解析服务，提升安全性  

# 配置文件内容说明：
```json
{
    "bind":{             // Socket 监听配置
        "udp":  ":53",   // 监听的 UDP 端口
        "http": ":8080"  // 监听的 HTTP 端口
    },
    "forwarders" : {     // 远程 DNS 服务器组，用于不同域名转发到不同的服务器组
        "normal":["223.5.5.5:53", "223.6.6.6:53", "119.29.29.29:53", "182.254.116.116:53", "101.226.4.6:53", "114.114.114.114:53", "114.114.115.115:53", "202.67.240.222:53", "203.80.96.10:53", "202.45.84.58:53"],
        "gfw":["74.82.42.42:53", "107.150.40.234:53", "162.211.64.20:53", "50.116.23.211:53", "50.116.40.226:53", "37.235.1.174:53", "37.235.1.177:53", "8.8.8.8:53", "8.8.4.4:53", "208.67.222.222:53", "208.67.220.220:53", "8.26.56.26:53", "84.200.69.80:53"]
    },
    "rules":{            // 转发规则，域名对应的服务器组，default 表示默认转发组。格式为：domain:group。如：imohe.com:normal, google.com:gfw, facebook.com:gfw
        "default": "normal"
    },
    "filters": [         // 查询过滤规则，暂时未实现。计划用于域名查询过滤，如：过滤广告域名
        {
            "host": "facebook.com",
            "type": "AAAA",
            "matching": "contains"
        }
    ],
    "mapper": [          // 域名与查询结果映射，可以用于内部非公开的域名解析服务
        "www.imohe.com:192.168.1.1", 
        ".demo.imohe.com:192.168.1.2", 
        "git.imohe.com:192.168.1.3"
    ],
    "logger": {         // 日志记录
        "Level":"debug",
        "Access":true,
        "Runtime":true
    }
}
```

# 后期开发计划：  
1、补上单元测试代码  
2、支持 SSL 证书，提升安全性  
3、对查询比较频繁的域名通过后台线程方式自动更新，以提升整体的性能  
4、加强域名与查询结果映射，实现完整的 DNS 查询支持（目前只支持 ipv4 的 A 记录查询）  

# 开发环境简单的性能测试：  
```bash
goos: windows  
goarch: amd64  
pkg: imohe/dnsproxy  
BenchmarkDig-4             10000            178410 ns/op  
PASS  
ok      imohe/dnsproxy  2.181s  
```