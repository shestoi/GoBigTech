//go:build e2e

package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	// TODO: проверь путь до pb
	inventorypb "github.com/shestoi/GoBigTech/services/inventory/v1"

	// TODO: проверь пути до handler/service/repo
	invhandler "github.com/shestoi/GoBigTech/services/inventory/internal/api/grpc"
	invrepo "github.com/shestoi/GoBigTech/services/inventory/internal/repository/mongo"
	invservice "github.com/shestoi/GoBigTech/services/inventory/internal/service"
)

func TestInventory_E2E_ReserveStock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1) Поднимаем MongoDB контейнер
	// Вариант: без auth (быстрее и надежнее для первого e2e)
	mongoC, err := mongodb.RunContainer(ctx,
		tc.WithImage("mongo:6"),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, mongoC.Terminate(ctx)) }()

	mongoURI, err := mongoC.ConnectionString(ctx)
	require.NoError(t, err)

	// 2) Подключаемся к Mongo как клиент и готовим коллекцию
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)
	defer func() { _ = client.Disconnect(ctx) }()

	// Ждём готовности MongoDB (ping с retry)
	var pingErr error
	for i := 0; i < 20; i++ {
		pingErr = client.Database("admin").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
		if pingErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, pingErr, "MongoDB did not become ready in time")

	dbName := "inventory"
	db := client.Database(dbName)
	col := db.Collection("inventory")

	// очистка на всякий
	_, _ = col.DeleteMany(ctx, bson.M{})

	_, err = col.InsertOne(ctx, bson.M{
		"product_id": "product-123",
		"stock":      int32(42),
		"updated_at": time.Now(),
	})
	require.NoError(t, err)

	// 3) Поднимаем Inventory gRPC сервер внутри теста (реальные repo+service+handler)
	repo := invrepo.NewRepository(client, dbName)
	svc := invservice.NewInventoryService(repo)
	h := invhandler.NewHandler(svc)

	grpcSrv := grpc.NewServer()
	inventorypb.RegisterInventoryServiceServer(grpcSrv, h)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()

	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	// 4) gRPC клиент
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	c := inventorypb.NewInventoryServiceClient(conn)

	// 5) success кейс: 42 - 10 = 32
	_, err = c.ReserveStock(ctx, &inventorypb.ReserveStockRequest{
		ProductId: "product-123",
		Quantity:  10,
	})
	require.NoError(t, err)

	var doc struct {
		ProductID string `bson:"product_id"`
		Stock     int32  `bson:"stock"`
	}
	err = col.FindOne(ctx, bson.M{"product_id": "product-123"}).Decode(&doc)
	require.NoError(t, err)
	require.Equal(t, int32(32), doc.Stock)

	// 6) fail кейс: резерв 1000 не должен уменьшить stock
	resp, err := c.ReserveStock(ctx, &inventorypb.ReserveStockRequest{
		ProductId: "product-123",
		Quantity:  1000,
	})
	require.NoError(t, err) // по вашей логике это может быть success=false, но без ошибки
	require.False(t, resp.Success)

	err = col.FindOne(ctx, bson.M{"product_id": "product-123"}).Decode(&doc)
	require.NoError(t, err)
	require.Equal(t, int32(32), doc.Stock)
}
