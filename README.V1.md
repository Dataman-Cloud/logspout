# logspout
数人云日志采集项目
##第一版
###1.编译项目
```
git clone git@github.com:Dataman-Cloud/logspout.git
cd logspout && docker build -t dataman/logspout .
```
###2.1运行项目
```
docker run -d --name="omega-logcollection" --net host -v /var/run/docker.sock:/tmp/docker.sock registry.dataman.io/logspout  #没有指定发送的地址使用默认
docker run -d --env HURL="tcp://123.59.58.58:5002" --name="omega-logcollection" --net host -v /var/run/docker.sock:/tmp/docker.sock registry.dataman.io/logspout:omega.v0.1  #使用自己指定的发段地址
```
##第二版
支持推送向kafka
```
如果server使用的kafka把连接地址改成kafka://x.x.x.x:9092,并且加入环境变量TOPIC=dataman-cloud加入自己的主题
```
