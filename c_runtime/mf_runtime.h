#ifndef _MF_RUNTIME_H
#define _MF_RUNTIME_H

typedef long int mf_number;

typedef enum
{
	TAG_NUMBER = -3,
	TAG_CLOSURE = -2,
	TAG_APP = -1,
	TAG_TUPLE = 0	// Uses length of tuple here!
} mf_tag;

typedef struct
{
	mf_tag tag;
} mf_value;

typedef struct
{
	mf_tag tag;
	long int number;
} mf_boxed_number;

typedef struct
{
	mf_tag tag;
	mf_value *func;
	mf_value *arg;
	mf_value *result;
} mf_app;

typedef mf_value *(*mf_func)(mf_value *arg, mf_value **upvalues);

typedef struct
{
	mf_tag tag;
	mf_func func;
	mf_value *upvalues[];
} mf_closure;

typedef struct
{
	mf_tag tag;
	mf_value *tuple[];
} mf_tuple;


mf_value *box_number(mf_number number);
mf_number unbox_number(mf_value *value);
mf_closure *make_closure(mf_func func, int num_upvalues);
mf_value *make_app(mf_value *func, mf_value *arg);
mf_tuple *make_tuple(int length);

void error(const char *message);

void init(void);
mf_value *reduce(mf_value *value);

#endif // _MF_RUNTIME_H
