#include <stdio.h>
#include "mf_runtime.h"

#include "gc.h"


mf_value *add_anon(mf_value *arg, mf_value **upvalues)
{
	mf_number ub_arg = unbox_number(arg);
	mf_number ub_up0 = unbox_number(upvalues[0]);

	return box_number(ub_up0 + ub_arg);
}

mf_value *add(mf_value *arg, mf_value **upvalues)
{
	mf_closure *closure = make_closure(add_anon, 1);
	closure->func = add_anon;
	closure->upvalues[0] = arg;

	return (mf_value*) closure;
}

int main(void)
{
	GC_INIT();
	
	mf_value *n = box_number(100);
	mf_value *m = box_number(50);

	mf_value *add_closure = (mf_value *) make_closure(add, 0);
	mf_value *app = make_app(add_closure, n);
	app = make_app(app, m);

	mf_value *result = reduce(app);

	return unbox_number(result);
}