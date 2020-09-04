#!/usr/bin/expect -f

set passwd "wxhlzlzyh"

#spawn scp ./kortho root@182.61.186.204:/root/bfttest
#expect "*password:"
#send "$passwd\r"
#interact

#spawn scp ./kortho root@182.61.177.227:/root/bfttest
#expect "*password:"
#send "$passwd\r"
#interact

spawn scp ./kortho root@106.12.73.41:/root/bfttest
expect "*password:"
send "$passwd\r"
interact

#spawn scp ./kortho root@182.61.184.64:/root/bfttest
#expect "*password:"
#send "$passwd\r"
#interact

