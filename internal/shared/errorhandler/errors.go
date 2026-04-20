package errorhandler

import (
	"golang-project-boilerplate/internal/model"
	"golang-project-boilerplate/internal/shared/constant"
)

func ErrorPanic(err error) *model.ErrorData {
	return &model.ErrorData{
		Code:    constant.InternalServerError,
		Message: constant.RecoverMsgErr,
		Error:   err,
	}
}

func ErrorInvalidRequest(err error) *model.ErrorData {
	return &model.ErrorData{
		Code:    constant.BadRequestError,
		Message: constant.BadRequestMsgErr,
		Error:   err,
	}
}

func ErrorNotFound(err error) *model.ErrorData {
	return &model.ErrorData{
		Code:    constant.NotfoundError,
		Message: constant.NotFoundMsgErr,
		Error:   err,
	}
}

func ErrorDB(err error) *model.ErrorData {
	return &model.ErrorData{
		Code:    constant.InternalServerError,
		Message: constant.DBMsgErr,
		Error:   err,
	}
}

func ErrorInvalidToken(err error) *model.ErrorData {
	return &model.ErrorData{
		Code:    constant.Unauthorized,
		Message: constant.InvalidTokenMsgErr,
		Error:   err,
	}
}
