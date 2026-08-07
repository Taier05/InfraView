package service

func validListPageSize(value int) bool {
	switch value {
	case 20, 50, 100, 500:
		return true
	default:
		return false
	}
}
