package storage

import (
	"backend/models"
	"fmt"

	"gorm.io/gorm"
)

type ProductInterface interface {
	List(limit, offset int, sort, filter string, direction string) ([]*models.Product, int, int, error)
	GetByID(id int) (*models.Product, error)
	Update(product *models.Product) (*models.Product, error)
	UpdateInventory(id int, inventory int) error
	Delete(id int) error
	Search(query string) ([]*models.Product, error)
}

type ProductDB struct {
	read  *gorm.DB
	write *gorm.DB
}

func NewProductDB(read, write *gorm.DB) ProductInterface {
	GetStorageInstance().AutoMigrate(&models.Product{})
	return &ProductDB{
		read:  read,
		write: write,
	}
}

func (p *ProductDB) List(limit, offset int, sort, filter string, direction string) ([]*models.Product, int, int, error) {
	var products []*models.Product
	query := p.read.Model(&models.Product{}).Where("is_deleted = false")

	if filter != "" {
		switch filter {
		case string(models.CategoryElectronics),
			string(models.CategoryClothing),
			string(models.CategoryHome),
			string(models.CategoryBooks),
			string(models.CategoryToys),
			string(models.CategoryMisc):
			query = query.Where("category = ?", filter)
		case "in_stock":
			query = query.Where("inventory > 0")
		default:
			return nil, 0, 0, fmt.Errorf("Invalid filter applied: %s", filter)
		}
	}

	if sort != "" {
		dir := "ASC"
		if direction == "desc" {
			dir = "DESC"
		}
		switch sort {
		case "name":
			query = query.Order(fmt.Sprintf("name %s", dir))
		case "price":
			query = query.Order(fmt.Sprintf("price %s", dir))
		default:
			return nil, 0, 0, fmt.Errorf("Invalid sort applied: %s", sort)
		}
	}

	// count total products
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	query = query.Order("updated_at DESC").Order("id ASC").Limit(limit)

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&products).Error; err != nil {
		return nil, 0, 0, err
	}

	nextOffset := offset + len(products)

	return products, int(total), nextOffset, nil
}

func (p *ProductDB) GetByID(id int) (*models.Product, error) {
	var product models.Product
	if err := p.read.Where("id = ?", id).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (p *ProductDB) Update(product *models.Product) (*models.Product, error) {
	result := p.write.Model(&models.Product{}).Omit("created_at", "updated_at").Where("id = ?", product.ID).Updates(product)
	if result.Error != nil {
		return nil, result.Error
	}

	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var updatedProduct *models.Product
	if err := p.read.Where("id = ?", product.ID).First(&product).Error; err != nil {
		return nil, err
	}

	return updatedProduct, nil
}

func (p *ProductDB) UpdateInventory(id int, inventory int) error {
	result := p.write.Model(&models.Product{}).Where("id = ?", id).Update("inventory", inventory)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (p *ProductDB) Delete(id int) error {
	result := p.write.Model(&models.Product{}).Where("id = ?", id).Update("is_deleted", true)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (p *ProductDB) Search(query string) ([]*models.Product, error) {
	var products []*models.Product

	err := p.read.Raw(`
    SELECT * FROM products
    WHERE
        (
            similarity(name, ?) >= 0.1 OR
            similarity(description, ?) >= 0.1 OR
            similarity(category_to_text(category), ?) >= 0.4
        )
        AND is_deleted = false
    ORDER BY
        GREATEST(
            similarity(name, ?),
            similarity(description, ?),
            similarity(category_to_text(category), ?)
        ) DESC`,
		query, query, query, query, query, query,
	).Scan(&products).Error

	return products, err
}
