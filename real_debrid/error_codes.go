package real_debrid

var errorCodes = map[int]string{
	-1: "internal error",
	1:  "missing parameter",
	2:  "bad parameter value",
	3:  "unknown method",
	4:  "method not allowed",
	5:  "slow down",
	6:  "ressource unreachable",
	7:  "resource not found",
	8:  "bad token",
	9:  "permission denied",
	10: "two-factor authentication needed",
	11: "two-factor authentication pending",
	12: "invalid login",
	13: "invalid password",
	14: "account locked",
	15: "account not activated",
	16: "unsupported hoster",
	17: "hoster in maintenance",
	18: "hoster limit reached",
	19: "hoster temporarily unavailable",
	20: "hoster not available for free users",
	21: "too many active downloads",
	22: "ip address not allowed",
	23: "traffic exhausted",
	24: "file unavailable",
	25: "service unavailable",
	26: "upload too big",
	27: "upload error",
	28: "file not allowed",
	29: "torrent too big",
	30: "torrent file invalid",
	31: "action already done",
	32: "image resolution error",
}