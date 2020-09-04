#!/bin/bash

NAME="kortho"

while :
do
	sleep 30
	PID=`ps -ef |grep "$NAME" | grep -v "grep" | awk '{print $2}'` 

	if [ $? -eq 0 ]; then
		if test -z "$PID"; then
			echo "$NAME not exit,restart now..."
			nohup ./kortho >> ./logs/control.log 2>&1 &
		else
			echo "$NAME $PID runs ok."
		fi	
	else
		echo "find process $NAME failed."
	fi
		
done


