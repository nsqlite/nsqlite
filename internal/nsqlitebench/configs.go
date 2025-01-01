package nsqlitebench

// benchmarksConfig holds all parameters for each benchmark.
type benchmarksConfig struct {
	benchmarkSimpleConfig
	benchmarkComplexConfig
	benchmarkManyConfig
	benchmarkLargeConfig
}

func getMattnConfig() benchmarksConfig {
	return benchmarksConfig{
		benchmarkSimpleConfig: benchmarkSimpleConfig{
			insertXUsers:     1_000_000,
			insertGoroutines: 1,
		},

		benchmarkComplexConfig: benchmarkComplexConfig{
			insertXUsers:              200,
			insertYArticlesPerUser:    100,
			insertZCommentsPerArticle: 20,
			insertGoroutines:          1,
		},

		benchmarkManyConfig: benchmarkManyConfig{
			insertXUsers:     1_000,
			queryUsersYTimes: 1_000,
			insertGoroutines: 1,
			queryGoroutines:  1,
		},

		benchmarkLargeConfig: benchmarkLargeConfig{
			insertXUsers:     10_000,
			insertYBytes:     10_000,
			insertGoroutines: 1,
		},
	}
}

func getNsqliteConfig() benchmarksConfig {
	mattnConfig := getMattnConfig()
	mattnConfig.benchmarkSimpleConfig.insertGoroutines = 10
	mattnConfig.benchmarkComplexConfig.insertGoroutines = 10
	mattnConfig.benchmarkManyConfig.insertGoroutines = 10
	mattnConfig.benchmarkManyConfig.queryGoroutines = 10
	mattnConfig.benchmarkLargeConfig.insertGoroutines = 10
	return mattnConfig
}
