package model


// Returns a list of values of the headers with the corresponding key in HttpReqDatum
func (m *HttpReqDatum) GetHeaderValues(key string) []string {
	res := make([]string, 0)
	if m != nil {
		for _,h := range m.Headers {
			if h.Key == key {
				res = append(res, h.Value)
			}
		}
	}
	return res
}

// Returns the first header with the corresponding key in the HttpReqDatum, or an empty string if not found
func (m *HttpReqDatum) GetHeader(key string) string {
	values := m.GetHeaderValues(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// Returns a list of values of the headers with the corresponding key in HttpRespDatum
func (m *HttpRespDatum) GetHeaderValues(key string) []string {
	res := make([]string, 0)
	if m != nil {
		for _,h := range m.Headers {
			if h.Key == key {
				res = append(res, h.Value)
			}
		}
	}
	return res
}

// Returns the first header with the corresponding key in the HttpRespDatum, or an empty string if not found
func (m *HttpRespDatum) GetHeader(key string) string {
	values := m.GetHeaderValues(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}
