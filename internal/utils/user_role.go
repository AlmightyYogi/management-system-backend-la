package utils

func RoleNameFromID(id uint) string {
	switch id {
	case 1:
		return "Administrator"
	case 2:
		return "IT Agent"
	case 3:
		return "User"
	case 4:
		return "Guest"
	default:
		return "End User"
	}
}