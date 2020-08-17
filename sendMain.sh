#!/usr/bin/expect -f

set passwd "wxhlzlzyh"

#spawn scp ./main root@106.12.94.134:/root/chain
#expect "*password:"
#send "$passwd\r"
#interact

#spawn scp ./main root@106.12.9.134:/root/chain
#expect "*password:"
#send "$passwd\r"
#interact

#spawn scp ./main root@106.12.88.252:/root/chain
#expect "*password:"
#send "$passwd\r"
#interact

spawn scp ./kortho root@106.12.186.114:/root/bfttest
expect "*password:"
send "$passwd\r"
interact

#spawn scp ./kortho root@106.12.186.120:/root/tmpchain
#expect "*password:"
#send "$passwd\r"
#interact

spawn scp ./kortho root@182.61.177.227:/root/bfttest
expect "*password:"
send "$passwd\r"
interact









