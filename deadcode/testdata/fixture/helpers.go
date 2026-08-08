package main

// acceptanceHelper is called only by TestAccFixture_basic: dead by default, live when
// treat-functions-as-used roots ^TestAcc.
func acceptanceHelper() {}

// unitOnlyHelper is called only by the TestUnitHelper unit test: dead by default and
// still dead under treat-functions-as-used ^TestAcc (unit tests are not entry points),
// live only with test: true.
func unitOnlyHelper() {}
