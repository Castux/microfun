#ifndef _MF_RUNTIME_H
#define _MF_RUNTIME_H

typedef enum
{
	TAG_NUMBER = -3,
	TAG_CLOSURE = -2,
	TAG_APP = -1,
	TAG_TUPLE = 0	// Uses length of tuple here!
} mf_tag;

typedef void (*mf_func)(mf_value arg, mf_func *closure);

typedef struct
{
	mf_tag tag;
} *mf_value;

typedef struct
{
	mf_tag tag;
	long int number;
} mf_number;

typedef struct
{
	mf_tag tag;
	mf_value func;
	mf_value arg;
} mf_app;

typedef struct
{
	mf_tag tag;
	mf_func func;
	mf_value upvalues[];
} mf_closure;

typedef struct
{
	mf_tag tag;
	mf_value tuple[];
} mf_tuple;


mf_value make_number(long int number);
mf_closure *make_closure(mf_func func, int num_upvalues);
mf_value make_app(mf_value func, mf_value arg);
mf_tuple *make_tuple(int length);

void init(int size);
void push(mf_value value);
mf_value peek(int i);

void error(const char* message);
void reduce(mf_value value);



#endif // _MF_RUNTIME_H
