package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/shestoi/GoBigTech/services/inventory/internal/repository"
)

// InventoryDocument представляет документ в коллекции MongoDB
type InventoryDocument struct {
	ProductID string    `bson:"product_id"`
	Stock     int32     `bson:"stock"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// Repository реализует InventoryRepository используя MongoDB
type Repository struct {
	client *mongo.Client
	db     *mongo.Database
	col    *mongo.Collection
}

// NewRepository создаёт новый MongoDB репозиторий
// Создаёт уникальный индекс на product_id при инициализации
func NewRepository(client *mongo.Client, dbName string) *Repository {
	db := client.Database(dbName)
	col := db.Collection("inventory")

	// Создаём уникальный индекс на product_id
	// Это гарантирует, что каждый товар будет иметь только один документ
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "product_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Создаём индекс (если уже существует - игнорируем ошибку)
	_, _ = col.Indexes().CreateOne(ctx, indexModel)

	return &Repository{
		client: client,
		db:     db,
		col:    col,
	}
}

// GetStock получает количество товара из MongoDB
// Возвращает ErrNotFound, если товар не найден
// Service слой обработает ErrNotFound и вернёт default=42
func (r *Repository) GetStock(ctx context.Context, productID string) (int32, error) {
	var doc InventoryDocument
	err := r.col.FindOne(ctx, bson.M{"product_id": productID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, repository.ErrNotFound
		}
		return 0, err
	}

	return doc.Stock, nil
}

// ReserveStock резервирует товар на складе атомарно
// Использует FindOneAndUpdate для атомарной проверки и обновления
// Логика: уменьшить stock на quantity, если stock >= quantity
// Возвращает true, если резервирование успешно, false если недостаточно товара
func (r *Repository) ReserveStock(ctx context.Context, productID string, quantity int32) (bool, error) {
	// Атомарная операция: найти документ с product_id и stock >= quantity,
	// затем уменьшить stock на quantity и обновить updated_at
	filter := bson.M{
		"product_id": productID,
		"stock":      bson.M{"$gte": quantity}, // stock >= quantity
	}

	update := bson.M{
		"$inc": bson.M{"stock": -quantity},           // уменьшить stock на quantity
		"$set": bson.M{"updated_at": time.Now()},     // обновить updated_at
	}

	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After) // вернуть документ после обновления

	var updatedDoc InventoryDocument
	err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Документ не найден или stock < quantity
			// Это означает: либо товара нет, либо недостаточно товара
			// Возвращаем false (недостаточно товара), но не ErrNotFound
			// Service слой обработает это как "недостаточно товара"
			return false, nil
		}
		return false, err
	}

	// Резервирование успешно
	return true, nil
}


