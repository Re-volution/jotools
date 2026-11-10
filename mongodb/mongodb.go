package mongodb

import (
	"context"
	"errors"
	"github.com/Re-volution/jotools/chantypes"
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/Re-volution/ltime"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"runtime/debug"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongodbManger struct {
	monclient        *mongo.Client
	playerColl       *mongo.Collection
	monLogCollClient *mongo.Collection
	ch               chan *msg
	clch             chan bool
	waitdata2        map[int][]interface{}
	waitUpdate       map[int64]*UpdateByKey
	waitWriteT       int64
	waitUpdateT      int64
	writePool        chan map[int64]*UpdateByKey
	writeWG          sync.WaitGroup
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
	if err = mm.monclient.Ping(ctx, nil); err != nil {
		_ = mm.monclient.Disconnect(context.Background())
		return err
	}
	mm.playerColl = mm.monclient.Database(dbname).Collection(collname)
	mm.monLogCollClient = mm.monclient.Database(dbcname).Collection(collname)
	mm.ch = make(chan *msg, 10240)
	mm.clch = make(chan bool, 1)
	mm.waitdata2 = make(map[int][]interface{})
	mm.waitUpdate = make(map[int64]*UpdateByKey)
	now := ltime.GetS()
	mm.waitWriteT = now + 5
	mm.waitUpdateT = now + 5
	mm.writePool = make(chan map[int64]*UpdateByKey, 8)
	initWritePool()
	initIndex([]string{"uid"})
	recover_p.Go(mm.update)
	filelog.Log("mongodb启动完成")
	return nil
}

func initWritePool() {
	n := cap(mm.writePool)
	for i := 0; i < n; i++ {
		recover_p.Go(func() { updateByCh(mm.writePool) })
	}
	filelog.Log("db写入数据线程池初始化完毕,线程池数量:", n)
}

func initIndex(indexs []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, v := range indexs {
		indexModel := mongo.IndexModel{
			Keys:    bson.D{{Key: v, Value: int32(1)}},
			Options: options.Index().SetUnique(true),
		}
		if _, err := mm.playerColl.Indexes().CreateOne(ctx, indexModel); err != nil {
			filelog.Debug(err, v)
		}
	}
}

func Close() {
	if mm == nil || mm.ch == nil {
		return
	}
	chantypes.TryWriteChan(mm.ch, &msg{ty: TCLOSE}, true)
	filelog.Log("准备写入mongodb缓存日志")
	select {
	case <-mm.clch:
		filelog.Log("日志写入结束,mongodb关闭,写入:true")
	case <-time.After(5 * time.Second):
		filelog.Log("日志写入结束,mongodb关闭,写入:false")
	}
	done := make(chan struct{})
	go func() {
		mm.writeWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		filelog.Error("mongodb close wait write timeout")
	}
	if mm.monclient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mm.monclient.Disconnect(ctx); err != nil {
			filelog.Error("mongodb disconnect:", err)
		}
	}
}

func (mm *mongodbManger) update() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case data := <-mm.ch:
			if data == nil {
				continue
			}
			stop := false
			func() {
				defer func() {
					if e := recover(); e != nil {
						filelog.Error("mongodb handle panic:", e, "\r\n", string(debug.Stack()))
					}
				}()
				stop = mm.handle(data)
			}()
			if stop {
				return
			}
		case <-ticker.C:
			mm.flushPending()
		}
	}
}

func Insert(ty int, data interface{}) {
	if mm == nil {
		filelog.Error("mongodb not init")
		return
	}
	chantypes.TryWriteChan(mm.ch, &msg{ty, data}, true)
}

func (mm *mongodbManger) handle(data *msg) bool {
	switch data.ty {
	case TINSERTONE, TINSERTMANY:
		mm.insert2(data)
	case TUPDATE:
		ub, ok := data.data.(*UpdateByKey)
		if !ok || ub == nil {
			filelog.Error("TUPDATE data type error")
			return false
		}
		updatePool(ub)
	case TCLOSE:
		mm.flushLogs()
		if len(mm.waitUpdate) > 0 {
			batch := mm.waitUpdate
			mm.waitUpdate = make(map[int64]*UpdateByKey)
			if err := update(batch); err != nil {
				filelog.Error("close update err:", err)
			}
		}
		select {
		case mm.clch <- true:
		default:
		}
		return true
	}
	return false
}

func (mm *mongodbManger) insert2(idata *msg) {
	const ty = TINSERTMANY
	if idata.ty == TINSERTMANY {
		rows, ok := idata.data.([]interface{})
		if !ok {
			filelog.Error("TINSERTMANY data type error")
			return
		}
		mm.waitdata2[ty] = append(mm.waitdata2[ty], rows...)
	} else {
		mm.waitdata2[ty] = append(mm.waitdata2[ty], idata.data)
	}
	if len(mm.waitdata2[ty]) > 1000 {
		insert2(mm.waitdata2[ty])
		mm.waitdata2[ty] = nil
	}
}

func (mm *mongodbManger) flushPending() {
	now := ltime.GetS()
	if mm.waitWriteT <= now {
		mm.flushLogs()
		mm.waitWriteT = now + 10
	}
	if mm.waitUpdateT <= now && len(mm.waitUpdate) > 0 {
		mm.flushUpdates()
	}
}

func (mm *mongodbManger) flushLogs() {
	for key, v := range mm.waitdata2 {
		if len(v) > 0 {
			insert2(v)
			mm.waitdata2[key] = nil
		}
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
		mm.flushUpdates()
	}
}

func (mm *mongodbManger) flushUpdates() {
	if len(mm.waitUpdate) == 0 {
		return
	}
	batch := mm.waitUpdate
	mm.waitUpdate = make(map[int64]*UpdateByKey)
	mm.waitUpdateT = ltime.GetS() + 5
	mm.writeWG.Add(1)
	select {
	case mm.writePool <- batch:
	default:
		mm.writeWG.Done()
		if err := update(batch); err != nil {
			filelog.Error("write db err:", err, "count:", len(batch))
		}
	}
}

func updateByCh(ch chan map[int64]*UpdateByKey) {
	for datas := range ch {
		func() {
			defer mm.writeWG.Done()
			defer func() {
				if e := recover(); e != nil {
					filelog.Error("update panic:", e, "\r\n", string(debug.Stack()))
				}
			}()
			if err := update(datas); err != nil {
				filelog.Error("write db err:", err, "count:", len(datas))
			}
		}()
	}
}

func update(datas map[int64]*UpdateByKey) error {
	if len(datas) == 0 {
		return nil
	}
	if mm.playerColl == nil {
		return errors.New("未产生数据库连接")
	}
	wData := make([]mongo.WriteModel, 0, len(datas))
	for _, data := range datas {
		one := mongo.NewUpdateOneModel().
			SetFilter(bson.D{{Key: "_id", Value: data.ID}}).
			SetUpdate(bson.M{"$set": data.Data}).
			SetUpsert(true)
		wData = append(wData, one)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := mm.playerColl.BulkWrite(ctx, wData, options.BulkWrite().SetOrdered(false)); err != nil {
		return err
	}
	filelog.Debug("write db success count:", len(wData))
	return nil
}

func insert2(data []interface{}) {
	if len(data) == 0 || mm.monLogCollClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := mm.monLogCollClient.InsertMany(ctx, data); err != nil {
		filelog.Error("InsertMany errorCode:", err, "count:", len(data))
	}
}

// GetSync 查到数据时第一个返回值为 false。没有文档时返回 true, nil。
func GetSync(filter interface{}, res interface{}, op ...*options.FindOneOptions) (bool, error) {
	if mm == nil || mm.playerColl == nil {
		return false, errors.New("未产生数据库连接")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := mm.playerColl.FindOne(ctx, filter, op...).Decode(res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return true, nil
		}
		filelog.Error("GetSync errorCode:", err, filter)
		return false, err
	}
	return false, nil
}
