#ifndef _MF_RUNTIME_H
#define _MF_RUNTIME_H

typedef enum
{
	TAG_NUMBER = -3,
	TAG_FUNC = -2,
	TAG_APP = -1,
	TAG_TUPLE = 0	// Uses length of tuple here!
} mf_tag;

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
	void (*func)(void);
} mf_func;

typedef struct
{
	mf_tag tag;
	mf_value tuple[];
} mf_tuple;


mf_value make_number(long int number);
mf_value make_func(void (*func)(void));
mf_value make_app(mf_value func, mf_value arg);
mf_tuple *make_tuple(int length);

void init(int size);
void push(mf_value value);

void error(const char* message);
void reduce(mf_value value);



#endif // _MF_RUNTIME_H
