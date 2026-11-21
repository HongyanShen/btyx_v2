package app

import (
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

type Error struct {
	Status  int
	Code    enum.ErrorCode
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func Invalid(m string) error {
	return &Error{
		Status:  400,
		Code:    enum.ErrorInvalidArgument,
		Message: m,
	}
}

func Conflict(m string) error {
	return &Error{
		Status:  409,
		Code:    enum.ErrorConflict,
		Message: m,
	}
}

func Forbidden(m string) error {
	return &Error{
		Status:  403,
		Code:    enum.ErrorForbidden,
		Message: m,
	}
}

func NotFound() error {
	return &Error{
		Status:  404,
		Code:    enum.ErrorNotFound,
		Message: "记录不存在或无访问权限",
	}
}

func WrapMissing(err error) error {
	if ent.IsNotFound(err) {
		return NotFound()
	}

	return err
}
