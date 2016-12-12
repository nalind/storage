package graphdriver

// ListDrivers returns a list of the registered drivers
func ListDrivers() (names []string) {
	drvs := []string{}
	for driver := range drivers {
		drvs = append(drvs, driver)
	}
	return drvs
}
