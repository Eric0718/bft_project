#!/bin/bash

NAME="kortho"

PID=`ps -ef |grep "$NAME" | grep -v "grep" | awk '{print $2}'` 

if [ $? -eq 0 ]; then
	if test -z "$PID"; then
		echo "$NAME not exit."
	else
		echo "$NAME id:$PID"
		kill -9 $PID

		if [ $? -eq 0 ]; then
			echo "kill $NAME successfully."
		else
			echo "kill $NAME failed."
		fi
	fi
fi


LNAME="listenKto.sh"

LPID=`ps -ef |grep "$LNAME" | grep -v "grep" | awk '{print $2}'` 

if [ $? -eq 0 ]; then
	if test -z "$LPID"; then
		echo "$LNAME not exit."
	else
		echo "$LNAME id:$LPID"
		kill -9 $LPID

		if [ $? -eq 0 ]; then
			echo "kill $LNAME successfully."
		else
			echo "kill $LNAME failed."
		fi
	fi
fi
#kill -9 $PID

#if [ $? -eq 0 ]; then
#	echo "kill $NAME successfully."
#else
#	echo "kill $NAME failed."
#fi

