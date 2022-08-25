// Code generated by mockery v2.10.4. DO NOT EDIT.

package mocks

import (
	context "context"

	module "github.com/odpf/entropy/core/module"
	mock "github.com/stretchr/testify/mock"

	resource "github.com/odpf/entropy/core/resource"
)

// ModuleDriver is an autogenerated mock type for the Driver type
type ModuleDriver struct {
	mock.Mock
}

type ModuleDriver_Expecter struct {
	mock *mock.Mock
}

func (_m *ModuleDriver) EXPECT() *ModuleDriver_Expecter {
	return &ModuleDriver_Expecter{mock: &_m.Mock}
}

// Plan provides a mock function with given fields: ctx, res, act
func (_m *ModuleDriver) Plan(ctx context.Context, res module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	ret := _m.Called(ctx, res, act)

	var r0 *module.Plan
	if rf, ok := ret.Get(0).(func(context.Context, module.ExpandedResource, module.ActionRequest) *module.Plan); ok {
		r0 = rf(ctx, res, act)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*module.Plan)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, module.ExpandedResource, module.ActionRequest) error); ok {
		r1 = rf(ctx, res, act)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ModuleDriver_Plan_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Plan'
type ModuleDriver_Plan_Call struct {
	*mock.Call
}

// Plan is a helper method to define mock.On call
//  - ctx context.Context
//  - res module.ExpandedResource
//  - act module.ActionRequest
func (_e *ModuleDriver_Expecter) Plan(ctx interface{}, res interface{}, act interface{}) *ModuleDriver_Plan_Call {
	return &ModuleDriver_Plan_Call{Call: _e.mock.On("Plan", ctx, res, act)}
}

func (_c *ModuleDriver_Plan_Call) Run(run func(ctx context.Context, res module.ExpandedResource, act module.ActionRequest)) *ModuleDriver_Plan_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(module.ExpandedResource), args[2].(module.ActionRequest))
	})
	return _c
}

func (_c *ModuleDriver_Plan_Call) Return(_a0 *module.Plan, _a1 error) *ModuleDriver_Plan_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

// Sync provides a mock function with given fields: ctx, res
func (_m *ModuleDriver) Sync(ctx context.Context, res module.ExpandedResource) (*resource.State, error) {
	ret := _m.Called(ctx, res)

	var r0 *resource.State
	if rf, ok := ret.Get(0).(func(context.Context, module.ExpandedResource) *resource.State); ok {
		r0 = rf(ctx, res)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*resource.State)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, module.ExpandedResource) error); ok {
		r1 = rf(ctx, res)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ModuleDriver_Sync_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Sync'
type ModuleDriver_Sync_Call struct {
	*mock.Call
}

// Sync is a helper method to define mock.On call
//  - ctx context.Context
//  - res module.ExpandedResource
func (_e *ModuleDriver_Expecter) Sync(ctx interface{}, res interface{}) *ModuleDriver_Sync_Call {
	return &ModuleDriver_Sync_Call{Call: _e.mock.On("Sync", ctx, res)}
}

func (_c *ModuleDriver_Sync_Call) Run(run func(ctx context.Context, res module.ExpandedResource)) *ModuleDriver_Sync_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(module.ExpandedResource))
	})
	return _c
}

func (_c *ModuleDriver_Sync_Call) Return(_a0 *resource.State, _a1 error) *ModuleDriver_Sync_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}
