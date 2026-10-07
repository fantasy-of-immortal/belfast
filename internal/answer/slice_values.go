package answer

func appendUniqueUint32(values []uint32, value uint32) []uint32 {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
