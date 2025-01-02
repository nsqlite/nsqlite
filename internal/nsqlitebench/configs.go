package nsqlitebench

// benchmarksConfig holds all parameters for each benchmark.
type benchmarksConfig struct {
	benchmarkSimpleConfig
	benchmarkComplexConfig
	benchmarkManyConfig
	benchmarkLargeConfig
}

func getMattnConfig() benchmarksConfig {
	insertGoroutines := 20
	queryGoroutines := 50

	return benchmarksConfig{
		benchmarkSimpleConfig: benchmarkSimpleConfig{
			insertXUsers:     1_000_000,
			insertGoroutines: insertGoroutines,
		},

		benchmarkComplexConfig: benchmarkComplexConfig{
			insertXUsers:              400,
			insertYArticlesPerUser:    100,
			insertZCommentsPerArticle: 20,
			insertGoroutines:          1,
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
