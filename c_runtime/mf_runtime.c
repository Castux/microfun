#include <stdlib.h>
#include <stdio.h>

#include "mf_runtime.h"
#include "gc.h"

mf_value make_number(long int number)
{
	mf_value value;
	value.tag = TAG_NUMBER;

	value.number = number;
	return value;
}

mf_value make_func(mf_func func)
{
	mf_value value;
	value.tag = TAG_FUNC;

	value.func = func;
	return value;
}

mf_value make_app(mf_value func, mf_value arg)
{
	mf_value value;
	value.tag = TAG_FUNC;

	value.app = GC_MALLOC(sizeof(struct mf_app_struct));
	value.app->func = func;
	value.app->arg = arg;

	return value;
}

mf_value make_tuple(int length)
{
	mf_value value;
	value.tag = length;

	value.tuple = GC_MALLOC(length * sizeof(mf_value));

	return value;
}

void error(const char *message)
{
	printf("microfun runtime error: %s\n", message);
	exit(EXIT_FAILURE);
}

// Stack

int stack_size;
mf_value *stack;
int stack_top;

void init(int size)
{
	stack_size = size;

	stack = GC_MALLOC(size * sizeof(mf_value));
	stack_top = -1;
}

void push(mf_value value)
{
	stack_top++;

	if(stack_top >= stack_size)
		error("stack overflow");

	stack[stack_top] = value;
}

void reduce(mf_value value)
{
	if(value.tag != TAG_APP)
		return;

	// Unwind the stack

	while(value.tag == TAG_APP)
	{
		push(value);
		value = value.app->func;
	}

	// At this point, value is the function to apply

	if(value.tag != TAG_FUNC)
		error("cannot apply non function");

	// The function reads the arguments, replacing all the application
	// with the result value

	value.func();

	// Update application

	// AAAAH WE CAN'T
}
