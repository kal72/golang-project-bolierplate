package utils

import (
	"golang-project-boilerplate/internal/model"
	"math"
)

// CalculatePage menghitung informasi paginasi berdasarkan total item dan item per halaman
func CalculatePage(totalItem int64, size int, page int) *model.PageMetadata {
	totalPage := int(math.Ceil(float64(totalItem) / float64(size)))

	return &model.PageMetadata{
		Page:      page,
		TotalItem: totalItem,
		Size:      size,
		TotalPage: totalPage,
	}
}

// CalculateLimitOffset menghitung nilai LIMIT dan OFFSET untuk paginasi SQL
func CalculateLimitOffset(page, size int) (limit, offset int) {
	if page < 1 {
		page = 1
	}

	offset = (page - 1) * size
	limit = size

	return limit, offset
}
