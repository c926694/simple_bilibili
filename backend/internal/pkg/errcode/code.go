package errcode

type Code int

const (
	Success         Code = 0
	InvalidParams   Code = 10001
	Unauthorized    Code = 10002
	Forbidden       Code = 10003
	NotFound        Code = 10004
	TooManyRequests Code = 10005
	InternalError   Code = 10500
	UsernameExists  Code = 11001
	EmailExists     Code = 11002
	PhoneExists     Code = 11003
	InvalidLogin    Code = 11004
	AccountBanned   Code = 11005
	UserNotFound    Code = 11006
	WrongPassword   Code = 11007
	InvalidRefresh  Code = 11008
)

func (c Code) Error() string {
	return c.Message()
}

func (c Code) Message() string {
	switch c {
	case Success:
		return "ok"
	case InvalidParams:
		return "参数错误"
	case Unauthorized:
		return "未登录或凭证无效"
	case Forbidden:
		return "无操作权限"
	case NotFound:
		return "资源或接口不存在"
	case TooManyRequests:
		return "请求过于频繁"
	case InternalError:
		return "服务内部错误"
	case UsernameExists:
		return "用户名已存在"
	case EmailExists:
		return "邮箱已被绑定"
	case PhoneExists:
		return "手机号已被绑定"
	case InvalidLogin:
		return "账号或密码错误"
	case AccountBanned:
		return "账号已被封禁"
	case UserNotFound:
		return "用户不存在"
	case WrongPassword:
		return "原密码错误"
	case InvalidRefresh:
		return "刷新凭证无效或已过期"
	default:
		return "未知错误"
	}
}
