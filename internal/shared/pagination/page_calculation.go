package shared

import (
	"golang-project-boilerplate/internal/model"
	"math"
)

// ToPageMetadata menghitung informasi paginasi berdasarkan total item dan item per halaman
func ToPageMetadata(totalItem int64, size int, page int) *model.PageMetadata {
	totalPage := int(math.Ceil(float64(totalItem) / float64(size)))

	return &model.PageMetadata{
		Page:      page,
		TotalItem: totalItem,
		Size:      size,
		TotalPage: totalPage,
	}
}

// GetLimitOffset konversi nilai page dan size menjadi nilai LIMIT dan OFFSET
func GetLimitOffset(page, size int) (limit, offset int) {
	if page < 1 {
		page = 1
	}

	offset = (page - 1) * size
	limit = size

	return limit, offset
}
