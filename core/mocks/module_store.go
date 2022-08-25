// Code generated by mockery v2.10.4. DO NOT EDIT.

package mocks

import (
	context "context"

	module "github.com/odpf/entropy/core/module"
	mock "github.com/stretchr/testify/mock"
)

// ModuleStore is an autogenerated mock type for the Store type
type ModuleStore struct {
	mock.Mock
}

type ModuleStore_Expecter struct {
	mock *mock.Mock
}

func (_m *ModuleStore) EXPECT() *ModuleStore_Expecter {
	return &ModuleStore_Expecter{mock: &_m.Mock}
}

// CreateModule provides a mock function with given fields: ctx, m
func (_m *ModuleStore) CreateModule(ctx context.Context, m module.Module) error {
	ret := _m.Called(ctx, m)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, module.Module) error); ok {
		r0 = rf(ctx, m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ModuleStore_CreateModule_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateModule'
type ModuleStore_CreateModule_Call struct {
	*mock.Call
}

// CreateModule is a helper method to define mock.On call
//  - ctx context.Context
//  - m module.Module
func (_e *ModuleStore_Expecter) CreateModule(ctx interface{}, m interface{}) *ModuleStore_CreateModule_Call {
	return &ModuleStore_CreateModule_Call{Call: _e.mock.On("CreateModule", ctx, m)}
}

func (_c *ModuleStore_CreateModule_Call) Run(run func(ctx context.Context, m module.Module)) *ModuleStore_CreateModule_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(module.Module))
	})
	return _c
}

func (_c *ModuleStore_CreateModule_Call) Return(_a0 error) *ModuleStore_CreateModule_Call {
	_c.Call.Return(_a0)
	return _c
}

// DeleteModule provides a mock function with given fields: ctx, urn
func (_m *ModuleStore) DeleteModule(ctx context.Context, urn string) error {
	ret := _m.Called(ctx, urn)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, urn)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ModuleStore_DeleteModule_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteModule'
type ModuleStore_DeleteModule_Call struct {
	*mock.Call
}

// DeleteModule is a helper method to define mock.On call
//  - ctx context.Context
//  - urn string
func (_e *ModuleStore_Expecter) DeleteModule(ctx interface{}, urn interface{}) *ModuleStore_DeleteModule_Call {
	return &ModuleStore_DeleteModule_Call{Call: _e.mock.On("DeleteModule", ctx, urn)}
}

func (_c *ModuleStore_DeleteModule_Call) Run(run func(ctx context.Context, urn string)) *ModuleStore_DeleteModule_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *ModuleStore_DeleteModule_Call) Return(_a0 error) *ModuleStore_DeleteModule_Call {
	_c.Call.Return(_a0)
	return _c
}

// GetModule provides a mock function with given fields: ctx, urn
func (_m *ModuleStore) GetModule(ctx context.Context, urn string) (*module.Module, error) {
	ret := _m.Called(ctx, urn)

	var r0 *module.Module
	if rf, ok := ret.Get(0).(func(context.Context, string) *module.Module); ok {
		r0 = rf(ctx, urn)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*module.Module)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, urn)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ModuleStore_GetModule_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetModule'
type ModuleStore_GetModule_Call struct {
	*mock.Call
}

// GetModule is a helper method to define mock.On call
//  - ctx context.Context
//  - urn string
func (_e *ModuleStore_Expecter) GetModule(ctx interface{}, urn interface{}) *ModuleStore_GetModule_Call {
	return &ModuleStore_GetModule_Call{Call: _e.mock.On("GetModule", ctx, urn)}
}

func (_c *ModuleStore_GetModule_Call) Run(run func(ctx context.Context, urn string)) *ModuleStore_GetModule_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *ModuleStore_GetModule_Call) Return(_a0 *module.Module, _a1 error) *ModuleStore_GetModule_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

// ListModules provides a mock function with given fields: ctx, project
func (_m *ModuleStore) ListModules(ctx context.Context, project string) ([]module.Module, error) {
	ret := _m.Called(ctx, project)

	var r0 []module.Module
	if rf, ok := ret.Get(0).(func(context.Context, string) []module.Module); ok {
		r0 = rf(ctx, project)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]module.Module)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ModuleStore_ListModules_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListModules'
type ModuleStore_ListModules_Call struct {
	*mock.Call
}

// ListModules is a helper method to define mock.On call
//  - ctx context.Context
//  - project string
func (_e *ModuleStore_Expecter) ListModules(ctx interface{}, project interface{}) *ModuleStore_ListModules_Call {
	return &ModuleStore_ListModules_Call{Call: _e.mock.On("ListModules", ctx, project)}
}

func (_c *ModuleStore_ListModules_Call) Run(run func(ctx context.Context, project string)) *ModuleStore_ListModules_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *ModuleStore_ListModules_Call) Return(_a0 []module.Module, _a1 error) *ModuleStore_ListModules_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

// UpdateModule provides a mock function with given fields: ctx, m
func (_m *ModuleStore) UpdateModule(ctx context.Context, m module.Module) error {
	ret := _m.Called(ctx, m)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, module.Module) error); ok {
		r0 = rf(ctx, m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ModuleStore_UpdateModule_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateModule'
type ModuleStore_UpdateModule_Call struct {
	*mock.Call
}

// UpdateModule is a helper method to define mock.On call
//  - ctx context.Context
//  - m module.Module
func (_e *ModuleStore_Expecter) UpdateModule(ctx interface{}, m interface{}) *ModuleStore_UpdateModule_Call {
	return &ModuleStore_UpdateModule_Call{Call: _e.mock.On("UpdateModule", ctx, m)}
}

func (_c *ModuleStore_UpdateModule_Call) Run(run func(ctx context.Context, m module.Module)) *ModuleStore_UpdateModule_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(module.Module))
	})
	return _c
}

func (_c *ModuleStore_UpdateModule_Call) Return(_a0 error) *ModuleStore_UpdateModule_Call {
	_c.Call.Return(_a0)
	return _c
}
