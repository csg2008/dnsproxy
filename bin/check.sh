#!/bin/bash
# 20170921 chen.s.g <chensg@imohe.com> dns proxy run status check
# crontab rule: * * * * * /usr/local/dnsproxy/bin/check.sh

bin=/usr/local/dnsproxy/bin/dnsproxy
cfg=/usr/local/dnsproxy/conf/proxy.json
log=/usr/local/dnsproxy/log/check.log

pid=$(ps -ef | grep ${bin} | grep -c -v grep)
if [ ${pid} -lt 1 ]; then
    touch ${log}
    ${bin} -c ${cfg} &> ${log} &
fi