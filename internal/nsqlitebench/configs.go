package nsqlitebench

// benchmarksConfig holds all parameters for each benchmark.
type benchmarksConfig struct {
	benchmarkSimpleConfig
	benchmarkComplexConfig
	benchmarkManyConfig
	benchmarkLargeConfig
}

func getMattnConfig() benchmarksConfig {
	insertGoroutines := 150
	queryGoroutines := 150

	return benchmarksConfig{
		benchmarkSimpleConfig: benchmarkSimpleConfig{
			insertXUsers:     100_000,
			queryYUsers:      200_000,
			insertGoroutines: insertGoroutines,
			queryGoroutines:  queryGoroutines,
		},

		benchmarkComplexConfig: benchmarkComplexConfig{
			insertXUsers:              400,
			insertYArticlesPerUser:    100,
			insertZCommentsPerArticle: 2,
			insertGoroutines:          insertGoroutines,
		},

		benchmarkManyConfig: benchmarkManyConfig{
			insertXUsers:     1_000,
			queryUsersYTimes: 1_000,
			insertGoroutines: insertGoroutines,
			queryGoroutines:  queryGoroutines,
		},

		benchmarkLargeConfig: benchmarkLargeConfig{
			insertXUsers:     10_000,
			insertYBytes:     10_000,
			insertGoroutines: insertGoroutines,
		},
	}
}

func getNsqliteConfig() benchmarksConfig {
	mattnConfig := getMattnConfig()
	return mattnConfig
}
