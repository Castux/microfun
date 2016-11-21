#include <stdlib.h>
#include <stdio.h>

#include "mf_runtime.h"
#include "gc.h"

mf_value *box_number(mf_number number)
{
	mf_boxed_number *value = GC_MALLOC(sizeof(mf_boxed_number));
	value->tag = TAG_NUMBER;
	value->number = number;
	
	return (mf_value*) value;
}

mf_number unbox_number(mf_value *value)
{
	if(value->tag != TAG_NUMBER)
		error("Cannot unbox non-number");

	return ((mf_boxed_number*)value)->number;
}

mf_closure *make_closure(mf_func func, int num_upvalues)
{
	mf_closure *value = GC_MALLOC(sizeof(mf_closure) + sizeof(mf_value[num_upvalues]));
	value->tag = TAG_CLOSURE;
	value->func = func;
	
	return value;
}

mf_value *make_app(mf_value *func, mf_value *arg)
{
	mf_app *value = GC_MALLOC(sizeof(mf_app));
	value->tag = TAG_APP;
	value->func = func;
	value->arg = arg;

	return (mf_value*) value;
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
mf_value **stack;
int stack_top;

void init(int size)
{
	stack_size = size;

	stack = GC_MALLOC(size * sizeof(mf_value*));
	stack_top = -1;
}

void push(mf_value *value)
{
	stack_top++;

	if(stack_top >= stack_size)
		error("stack overflow");

	stack[stack_top] = value;
}

mf_value *peek(int i)
{
	if(i > stack_top)
		error("peek underflow");

	return stack[stack_top - i];
}

void reduce(mf_value *value)
{
	if(value->tag != TAG_APP)
		return;

	// Unwind the stack

	while(value->tag == TAG_APP)
	{
		push(value);
		value = ((mf_app*) value)->func;
	}

	// At this point, value is the function to apply

	if(value->tag != TAG_CLOSURE)
		error("cannot apply non function");

	mf_value *arg = ((mf_app*) peek(1))->arg;
	mf_closure *closure = (mf_closure*) value;

	closure->func(arg, closure->upvalues);
}

