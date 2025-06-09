自建的一些工具代码，方便新开卡牌rpg类项目时使用

//读取配置文件
config.InitConfig()

//初始化读配置表
chJson.ReadCXJson()

//初始化redis
redis.InitRedisClient(config.DBConf.RedisAddr)

//初始化mongodb
err = mongodb.InitMongodb(config.DBConf.MongodbAddr)

//初始化cdk
cdkey.CDKeyInit()

//初始化链接
ConNet = net.Conn(config.GameConf.GtAddr+":"+config.GameConf.GtPort, GtHandle, GtOffline, GtSuccess, ctx)

//启动监听
WsNetM, _ = net.StartWSListen("/togame", config.GameConf.Port, PM.handleMsg, PM.offlineh, successh, ctx, false)


///////关闭流程
//关闭数据库
mongodb.Close()
