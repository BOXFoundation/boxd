package main

var (
	collAccounts      = 10
	collUnitAccounts  = 5
	circuAccounts     = 10
	circuUnitAccounts = 5
	tokenAccounts     = 3
	tokenUnitAccounts = 3
)

func loadConf() error {
	var err error
	collAccounts, err = GetIntCfgVal(10, "transaction_test", "collection_accounts")
	if err != nil {
		return err
	}
	logger.Infof("collAccounts: %d", collAccounts)

	collUnitAccounts, err = GetIntCfgVal(5, "transaction_test", "collection_unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("collUnitAccounts %d", collUnitAccounts)

	circuAccounts, err = GetIntCfgVal(10, "transaction_test", "circulation_accounts")
	if err != nil {
		return err
	}
	logger.Infof("circuAccounts %d", circuAccounts)

	circuUnitAccounts, err = GetIntCfgVal(5, "transaction_test", "circulationn_unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("circuUnitAccounts %d", circuUnitAccounts)

	tokenAccounts, err = GetIntCfgVal(10, "token_test", "accounts")
	if err != nil {
		return err
	}
	logger.Infof("tokenAccounts %d", tokenAccounts)

	tokenUnitAccounts, err = GetIntCfgVal(10, "token_test", "unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("tokenUnitAccounts %d", tokenUnitAccounts)

	return nil
}

// CollAccounts return collAccounts
func CollAccounts() int {
	return collAccounts
}

// CollUnitAccounts return collUnitAccounts
func CollUnitAccounts() int {
	return collUnitAccounts
}

// CircuAccounts return circuAccounts
func CircuAccounts() int {
	return circuAccounts
}

// CircuUnitAccounts return circuUnitAccounts
func CircuUnitAccounts() int {
	return circuUnitAccounts
}

// TokenAccounts return tokenAccounts
func TokenAccounts() int {
	return tokenAccounts
}

// TokenUnitAccounts return tokenUnitAccounts
func TokenUnitAccounts() int {
	return tokenUnitAccounts
}
