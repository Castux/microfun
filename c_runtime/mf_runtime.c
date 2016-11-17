#include <stdlib.h>
#include <stdio.h>

#include "mf_runtime.h"
#include "gc.h"

mf_value make_number(long int number)
{
	mf_number *value = GC_MALLOC(sizeof(mf_number));
	value->tag = TAG_NUMBER;
	value->number = number;
	
	return (mf_value) value;
}

mf_value make_func(void (*func)(void))
{
	mf_func *value = GC_MALLOC(sizeof(mf_func));
	value->tag = TAG_FUNC;
	value->func = func;
	
	return (mf_value) value;
}

mf_value make_app(mf_value func, mf_value arg)
{
	mf_app *value = GC_MALLOC(sizeof(mf_app));
	value->tag = TAG_APP;
	value->func = func;
	value->arg = arg;

	return (mf_value) value;
}

mf_tuple *make_tuple(int length)
{
	mf_tuple *value = GC_MALLOC(sizeof(mf_tuple) + sizeof(mf_value[length]));
	value->tag = length;
	
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
/*
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
*/
