nohup ./kortho >> ./logs/control.log 2>&1 &
nohup ./listenKto.sh >> logs/restartKto.log 2>&1 &
tail -f ./logs/control.log
