package constant

const (
	DateFormat = "2006-01-02 15:04:05"

	//response
	SuccessCode = "00"

	// error
	InternalServerError = 500
	BadRequestError     = 400
	NotfoundError       = 404
	Unauthorized        = 401
	MsgError            = "Terjadi kesalahan pada server"
	RecoverMsgErr       = MsgError + " - RE"
	DBMsgErr            = MsgError + " - DB"
	NotFoundMsgErr      = "Data tidak ditemukan"
	BadRequestMsgErr    = "Request tidak valid"
	InvalidTokenMsgErr  = "Token tidak valid"
)
