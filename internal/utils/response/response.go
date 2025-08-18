package response

import (
	"fmt"
	"golang-project-boilerplate/internal/model"
	"golang-project-boilerplate/internal/utils/constant"

	"github.com/gofiber/fiber/v2"
)

func ResponseSuccess(ctx *fiber.Ctx, data interface{}) error {
	return ctx.Status(fiber.StatusOK).
		JSON(model.Response{
			Status:  constant.SuccessCode,
			Data:    data,
			Message: "Success",
		})
}

func ResponseSuccessWithPaging(ctx *fiber.Ctx, data interface{}, paging model.PageMetadata) error {
	return ctx.Status(fiber.StatusOK).
		JSON(model.Response{
			Status:   constant.SuccessCode,
			Data:     data,
			Message:  "Success",
			Metadata: paging,
		})
}

func ResponseError(ctx *fiber.Ctx, err model.ErrorData) error {
	return ctx.Status(err.Code).
		JSON(model.Response{
			Status:  fmt.Sprint(err.Code),
			Data:    nil,
			Message: err.Message,
		})
}
