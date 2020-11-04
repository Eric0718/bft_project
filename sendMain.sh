#!/usr/bin/expect -f

set passwd "wxhlzlzyh"

spawn scp ./testBft root@106.12.94.134:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@106.12.9.134:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@106.12.88.252:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@106.12.186.114:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@106.12.186.120:/root/tmptestOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@182.61.177.227:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@106.12.73.41:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@106.12.176.28:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

spawn scp ./testBft root@106.13.188.227:/root/testOnlineBft
expect "*password:"
send "$passwd\r"
interact

