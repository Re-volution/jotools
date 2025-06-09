package mongodb

import (
	"context"
	"errors"
	"github.com/Re-volution/ltime"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
	"ysj/jotools/chantypes"
	"ysj/jotools/filelog"
	"ysj/jotools/recover_p"

	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongodbManger struct {
	monclient        *mongo.Client
	monLogCollClient *mongo.Collection //日志数据
	ch               chan *msg
	clch             chan bool
	waitdata2        map[int][]interface{}  //客户端上报数据
	waitUpdate       map[int64]*UpdateByKey //待更新的数据
	waitWriteT       int64
	waitUpdateT      int64
	writePool        chan map[int64]*UpdateByKey //数据库写入线程池
	readPool         chan *mongo.Collection      //数据库读取线程池
	Url              string
}

type msg struct {
	ty   int
	data interface{}
}

const (
	dbname   = "player"
	collname = "data"
	dbcname  = "cplayerLog"
)

const (
	TINSERTONE  = 1
	TUPDATE     = 2
	TINSERTMANY = 3
	TCLOSE      = 99
)

var mm *mongodbManger

// InitMongodb "mongodb://localhost:27017"
func InitMongodb(urlAddr string) error {
	mm = new(mongodbManger)
	mm.Url = urlAddr
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var err error
	mm.monclient, err = mongo.Connect(ctx, options.Client().ApplyURI(urlAddr))
	if err != nil {
		return err
	}
	err = mm.monclient.Ping(ctx, nil)
	if err != nil {
		return err
	}
	mm.monLogCollClient = mm.monclient.Database(dbcname).Collection(collname)
	mm.ch = make(chan *msg, 10240)
	mm.clch = make(chan bool, 0)
	mm.waitdata2 = make(map[int][]interface{})
	mm.waitUpdate = make(map[int64]*UpdateByKey)
	mm.waitWriteT = ltime.GetS() + 5

	mm.writePool = make(chan map[int64]*UpdateByKey, 8)
	mm.readPool = make(chan *mongo.Collection, 8)
	initWritePool()
	initIndex([]string{"uid"})

	recover_p.Go(mm.update)
	filelog.Log("mongodb启动完成")

	return nil
}

func getNewWriteTh() *mongo.Collection {
	monclient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mm.Url))
	if err != nil {
		filelog.Error("Connect err:", err)
		return nil
	}
	err = monclient.Ping(context.Background(), nil)
	if err != nil {
		filelog.Error("Ping err:", err)
		return nil
	}
	monCollClient := monclient.Database(dbname).Collection(collname)
	return monCollClient
}

func getReadCollByPool() *mongo.Collection {
	rp := <-mm.readPool
	if rp == nil {
		rp = getNewWriteTh()
	}
	return rp
}

func pushReadCollToPool(rp *mongo.Collection) {
	mm.readPool <- rp
}

func initWritePool() {
	for i := 0; i < cap(mm.writePool); i++ {
		recover_p.Go(func() { updateByCh(mm.writePool) })
	}
	filelog.Log("db写入数据线程池初始化完毕,线程池数量:", cap(mm.writePool))
	for i := 0; i < cap(mm.readPool); i++ {
		mm.readPool <- getNewWriteTh()
	}
	filelog.Log("db读取数据线程池初始化完毕,线程池数量:", cap(mm.readPool))
}

func initIndex(indexs []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for _, v := range indexs {
		nio := options.Index().SetUnique(true)
		indexModel := mongo.IndexModel{
			Keys:    bson.D{{v, int64(1)}},
			Options: nio,
		}
		rp := getReadCollByPool()
		if rp == nil {
			filelog.Error("db未取到相应链接")
			return
		}
		_, err := rp.Indexes().CreateOne(ctx, indexModel)
		if err != nil {
			filelog.Debug(err, v)
		}
		pushReadCollToPool(rp)
	}
	return
}

func Close() {
	chantypes.TryWriteChan[*msg](mm.ch, &msg{TCLOSE, nil}, true)
	filelog.Log("准备写入mongodb缓存日志")
	_, bo := chantypes.ReadChanByOverTime(mm.clch, 5)
	filelog.Log("日志写入结束,mongodb关闭,写入:", bo)
	return
}

func (mm *mongodbManger) update() {
	for {
		select {
		case data := <-mm.ch:
			mm.handle(data)
		}
	}
}

func Insert(ty int, data interface{}) {
	chantypes.TryWriteChan[*msg](mm.ch, &msg{ty, data}, true)
}

func (mm *mongodbManger) handle(data *msg) {
	//now := time.Now()
	switch data.ty {
	case TINSERTONE, TINSERTMANY:
		mm.insert2(data)
	case TUPDATE:
		updatePool(data.data.(*UpdateByKey))
	case TCLOSE:
		for key, v := range mm.waitdata2 {
			if len(v) > 0 {
				insert2(v)
				mm.waitdata2[key] = nil
			}
		}

		if len(mm.waitUpdate) > 0 {
			wp := getNewWriteTh()
			update(mm.waitUpdate, wp)
			mm.waitUpdate = make(map[int64]*UpdateByKey)
		}

		mm.clch <- true
	}

}

func (mm *mongodbManger) insert2(idata *msg) {
	var ty = TINSERTMANY
	if idata.ty == TINSERTMANY {
		mm.waitdata2[ty] = append(mm.waitdata2[ty], idata.data.([]interface{})...)
	} else {
		mm.waitdata2[ty] = append(mm.waitdata2[ty], idata.data)
	}

	if len(mm.waitdata2[ty]) > 1000 {
		insert2(mm.waitdata2[ty])
		mm.waitdata2[ty] = nil
	}

	if mm.waitWriteT < ltime.GetS() {
		for key, v := range mm.waitdata2 {
			if len(v) > 0 {
				insert2(v)
				mm.waitdata2[key] = nil
			}
		}
		mm.waitWriteT = ltime.GetS() + 10
	}
}

type UpdateByKey struct {
	Key  int64
	ID   primitive.ObjectID
	Data bson.Raw
	Bo   bool //是否立即写入
}

func updatePool(data *UpdateByKey) {
	mm.waitUpdate[data.Key] = data
	if len(mm.waitUpdate) > 30 || data.Bo {
		mm.writePool <- mm.waitUpdate
		mm.waitUpdateT = ltime.GetS() + 5
		mm.waitUpdate = make(map[int64]*UpdateByKey)
	}

	if mm.waitUpdateT < ltime.GetS() {
		mm.waitUpdateT = ltime.GetS() + 5
		if len(mm.waitUpdate) > 0 {
			mm.writePool <- mm.waitUpdate
			mm.waitUpdate = make(map[int64]*UpdateByKey)
		}
	}
}

func updateByCh(ch chan map[int64]*UpdateByKey) {
	wp := getNewWriteTh()
	for {
		select {
		case datas := <-ch:
			if wp == nil {
				wp = getNewWriteTh()
			}
			err := update(datas, wp)
			if err != nil {
				wp = getNewWriteTh()
				filelog.Error("write db err:", err, datas)
				//TODO:处理写入失败的数据
			}
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func update(datas map[int64]*UpdateByKey, wp *mongo.Collection) error {
	if len(datas) == 0 {
		filelog.Warring("update nil")
		return nil
	}
	var bo = new(bool)
	*bo = true
	var wData []mongo.WriteModel
	for _, data := range datas {
		one := &mongo.UpdateOneModel{
			Collation:    nil,
			Upsert:       bo,
			Filter:       bson.D{{"_id", data.ID}},
			Update:       bson.M{"$set": data.Data},
			ArrayFilters: nil,
			Hint:         nil,
		}
		wData = append(wData, one)
		filelog.Debug("write db success:", data.Key)
	}
	//无序在有些场景下可提高性能
	var bo2 = new(bool)
	*bo2 = false
	opt := &options.BulkWriteOptions{
		BypassDocumentValidation: nil,
		Comment:                  nil,
		Ordered:                  bo2,
		Let:                      nil,
	}

	_, e := wp.BulkWrite(context.Background(), wData, opt)
	if e != nil {
		filelog.Error("BulkWrite err:", e)
		return e
	} else {
		//	filelog.Debug("write success count:", *b, " ms:", (time.Now().UnixNano()-now)/int64(time.Millisecond), "totle:", len(wData))
	}
	return nil
}

func insert(data []interface{}, wp *mongo.Collection) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := wp.InsertMany(ctx, data)
	if err != nil {
		filelog.Error("InsertOne errorCode:", err, data)
		return
	}
}

// 客户端上报
func insert2(data []interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := mm.monLogCollClient.InsertMany(ctx, data)
	if err != nil {
		filelog.Error("InsertOne errorCode:", err, data)
		return
	}
}

// 直接获取
func GetSync(filter interface{}, res interface{}, op ...*options.FindOneOptions) (bool, error) {
	wp := getReadCollByPool()
	if wp == nil {
		pushReadCollToPool(wp)
		return false, errors.New("未产生数据库连接")
	}
	se := wp.FindOne(context.Background(), filter, op...)
	pushReadCollToPool(wp)
	err := se.Decode(res)
	if err != nil {
		if mongo.ErrNoDocuments == err {
			return true, nil
		}
		filelog.Error("GetSync errorCode:", err, filter)
		return false, err
	}

	return false, nil
}
