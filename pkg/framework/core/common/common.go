package common

import (
	"fmt"
	"github.com/ozontech/allure-go/pkg/framework/core/constants"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"testing"

	"github.com/ozontech/allure-go/pkg/allure"
	"github.com/ozontech/allure-go/pkg/framework/asserts_wrapper/helper"
	"github.com/ozontech/allure-go/pkg/framework/core/allure_manager/manager"
	"github.com/ozontech/allure-go/pkg/framework/core/allure_manager/testplan"
	"github.com/ozontech/allure-go/pkg/framework/provider"
)

type Common struct {
	provider.TestingT
	provider.Provider

	assert  provider.Asserts
	require provider.Asserts

	xSkip bool

	wg sync.WaitGroup
}

// NewT returns Common instance that implementing provider.T interface
func NewT(realT provider.TestingT) *Common {
	newT := &Common{TestingT: realT}
	newT.assert = helper.NewAssertsHelper(newT)
	newT.require = helper.NewRequireHelper(newT)
	return newT
}

func (c *Common) registerError(fullMessage string) {
	xSkipPrefix := "[XSkip]"
	result := c.GetResult()
	if result != nil && result.Status != allure.Broken {
		if c.xSkip {
			result.Name = fmt.Sprintf("%s%s", xSkipPrefix, result.Name)
			c.Skip(fullMessage)
		}
		result.Status = allure.Failed
		result.StatusDetails.Message = extractErrorMessages(fullMessage)
		result.StatusDetails.Trace = fmt.Sprintf("%s\n%s", result.StatusDetails.Trace, fullMessage)
	}
}

func (c *Common) safely(f func(result *allure.Result)) {
	if result := c.GetResult(); result != nil {
		f(result)
	}
}

func (c *Common) SetProvider(provider provider.Provider) {
	c.Provider = provider
}

// WG ...
func (c *Common) WG() *sync.WaitGroup {
	return &c.wg
}

// RealT returns instance of testing.T
func (c *Common) RealT() provider.TestingT {
	return c.TestingT
}

// Assert ...
func (c *Common) Assert() provider.Asserts {
	return c.assert
}

// Require ...
func (c *Common) Require() provider.Asserts {
	return c.require
}

// XSkip marks current test as XSkip that means that in case of assert fail this test will be marked skipped
func (c *Common) XSkip() {
	c.xSkip = true
}

// GetProvider ...
func (c *Common) GetProvider() provider.Provider {
	return c.Provider
}

// SkipOnPrint skips creating of report for current test
func (c *Common) SkipOnPrint() {
	c.GetResult().SkipOnPrint()
}

// LogStep ...
func (c *Common) LogStep(args ...interface{}) {
	c.Provider.Step(allure.NewSimpleStep(fmt.Sprintln(args...)))
	c.Log(args...)
}

// LogfStep ...
func (c *Common) LogfStep(format string, args ...interface{}) {
	c.Provider.Step(allure.NewSimpleStep(fmt.Sprintf(format, args...)))
	c.Logf(format, args...)
}

// Error ...
func (c *Common) Error(args ...interface{}) {
	fullMessage := fmt.Sprintf("%s", args...)
	c.registerError(fullMessage)
	c.TestingT.Error(args...)
}

// Errorf ...
func (c *Common) Errorf(format string, args ...interface{}) {
	fullMessage := fmt.Sprintf(format, args...)
	c.registerError(fullMessage)
	c.TestingT.Errorf(format, args...)
}

// Fatal ...
func (c *Common) Fatal(args ...interface{}) {
	fullMessage := fmt.Sprintf("%s", args...)
	c.registerError(fullMessage)
	c.TestingT.Fatal(args...)
}

// Fatalf ...
func (c *Common) Fatalf(format string, args ...interface{}) {
	fullMessage := fmt.Sprintf(format, args...)
	c.registerError(fullMessage)
	c.TestingT.Fatalf(format, args...)
}

// Name ...
func (c *Common) Name() string {
	if c.GetProvider() != nil && c.GetProvider().GetResult() != nil {
		return c.GetProvider().GetResult().Name
	}
	return c.TestingT.Name()
}

// Fail ...
func (c *Common) Fail() {
	c.GetProvider().GetResult().Status = allure.Failed
	c.TestingT.Fail()
}

// FailNow ...
func (c *Common) FailNow() {
	c.safely(func(result *allure.Result) {
		if result.Status != allure.Broken {
			result.Status = allure.Failed
		}
	})
	c.TestingT.FailNow()
}

// Skip ...
func (c *Common) Skip(args ...interface{}) {
	c.safely(func(result *allure.Result) {
		skipMessage := fmt.Sprintln(args...)
		if len(skipMessage) > 100 {
			result.StatusDetails.Message = skipMessage[:100]
		} else {
			result.StatusDetails.Message = skipMessage
		}
		result.StatusDetails.Trace = skipMessage
		result.Status = allure.Skipped
	})
	c.TestingT.Skip(args...)
}

// Skipf ...
func (c *Common) Skipf(format string, args ...interface{}) {
	c.safely(func(result *allure.Result) {
		skipMessage := fmt.Sprintf(format, args...)
		if len(skipMessage) > 100 {
			result.StatusDetails.Message = skipMessage[:100]
		} else {
			result.StatusDetails.Message = skipMessage
		}
		result.StatusDetails.Trace = skipMessage
		result.Status = allure.Skipped
	})
	c.TestingT.Skipf(format, args...)
}

// WithTestSetup ...
func (c *Common) WithTestSetup(setup func(provider.T)) {
	currentContext := c.GetProvider().ExecutionContext().GetName()
	if currentContext != constants.TestContextName {
		c.Logf("WithTestSetup will be skipped. Reason: wrong context. Expected: %s; Actual: %s", constants.TestContextName, currentContext)
		return
	}
	defer func() {
		rec := recover()
		if rec != nil {
			ctxName := currentContext
			errMsg := fmt.Sprintf("%s panicked: %v\n%s", ctxName, rec, debug.Stack())
			TestError(c, c.GetProvider(), currentContext, errMsg)
		}
	}()
	c.GetProvider().BeforeEachContext()
	defer c.GetProvider().TestContext()
	setup(c)
}

// WithTestTeardown ...
func (c *Common) WithTestTeardown(teardown func(provider.T)) {
	currentContext := c.GetProvider().ExecutionContext().GetName()
	if currentContext != constants.TestContextName {
		c.Logf("WithTestTeardown will be skipped. Reason: wrong context. Expected: %s; Actual: %s", constants.TestContextName, currentContext)
		return
	}
	defer func() {
		rec := recover()
		if rec != nil {
			ctxName := currentContext
			errMsg := fmt.Sprintf("%s panicked: %v\n%s", ctxName, rec, debug.Stack())
			TestError(c, c.GetProvider(), currentContext, errMsg)
		}
	}()
	c.GetProvider().AfterEachContext()
	defer c.GetProvider().TestContext()
	teardown(c)
}

// Run runs test body as test with passed tags
func (c *Common) Run(testName string, testBody func(provider.T), tags ...string) (res *allure.Result) {
	parentCallers := strings.Split(c.RealT().Name(), "/")
	suiteName := parentCallers[len(parentCallers)-1]

	c.TestingT.Run(testName, func(realT *testing.T) {
		var (
			testT = NewT(realT)

			packageName = c.Provider.GetSuiteMeta().GetPackageName()
			parentSuite = c.Provider.GetSuiteMeta().GetSuiteName()

			callers = strings.Split(realT.Name(), "/")
		)

		if result := c.Provider.GetTestMeta().GetResult(); result != nil {
			suiteName = result.Name
		}

		providerCfg := manager.NewProviderConfig().
			WithFullName(realT.Name()).
			WithPackageName(packageName).
			WithSuiteName(suiteName).
			WithRunner(callers[0])

		if parentSuite != "" && parentSuite != suiteName && parentSuite != callers[len(callers)-1] {
			providerCfg = providerCfg.WithParentSuite(parentSuite)
		}
		newProvider := manager.NewProvider(providerCfg)

		newProvider.NewTest(testName, packageName, tags...)
		if testPlan := testplan.GetTestPlan(); testPlan != nil {
			if !testPlan.IsSelected(newProvider.GetTestMeta().GetResult().TestCaseID, newProvider.GetResult().FullName) {
				realT.Skip("Test is not Selected in Test Plan")
			}
		}
		newProvider.TestContext()

		testT.SetProvider(newProvider)

		defer func() {
			res = testT.GetResult()
		}()

		// print test result
		defer func() {
			err := testT.Provider.FinishTest()
			if err != nil {
				testT.Error(err.Error())
			}
		}()

		defer func() {
			rec := recover()
			// wait for all test's async steps over
			testT.wg.Wait()
			if rec != nil {
				errMsg := fmt.Sprintf("Test panicked: %v\n%s", rec, debug.Stack())
				TestError(testT, testT.Provider, testT.Provider.ExecutionContext().GetName(), errMsg)
			}
		}()

		testT.Provider.TestContext()
		testBody(testT)
	})
	return
}

func (c *Common) SetRealT(realT provider.TestingT) {
	c.TestingT = realT
}

func copyLabels(input, target *allure.Result) *allure.Result {
	if input == nil || target == nil {
		return target
	}

	if epics := input.GetLabel(allure.Epic); len(epics) > 0 {
		target.SetLabel(epics[0])
	}

	if parentSuites := input.GetLabel(allure.ParentSuite); len(parentSuites) > 0 {
		target.SetLabel(parentSuites[0])
	}

	if leads := input.GetLabel(allure.Lead); len(leads) > 0 {
		target.SetLabel(leads[0])
	}

	if owners := input.GetLabel(allure.Owner); len(owners) > 0 {
		target.SetLabel(owners[0])
	}

	return target
}

func extractErrorMessages(output string) string {
	r := regexp.MustCompile(`Messages:(.*)`)
	result := strings.Trim(strings.TrimPrefix(r.FindString(output), "Messages:   "), " ")
	if result == "" {
		left := "\tError:"
		right := "\tTest:"
		r2 := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(left) + `(.*?)` + regexp.QuoteMeta(right))
		result = r2.FindString(output)
		result = strings.Trim(strings.TrimSuffix(result, "\tTest:"), " ")
	}
	if result == "" {
		return output
	}
	return result
}
