#ifndef _MF_RUNTIME_H
#define _MF_RUNTIME_H

struct mf_app_struct;
typedef struct mf_app_struct *mf_app;

struct mf_value_struct;
typedef struct mf_value_struct mf_value;

typedef mf_value *mf_tuple;

typedef void (*mf_func) (void);

typedef enum
{
	TAG_NUMBER = -3,
	TAG_FUNC = -2,
	TAG_APP = -1,
	TAG_TUPLE = 0	// Uses length of tuple here!
} mf_tag;

struct mf_value_struct
{
	mf_tag tag;
	union
	{
		long int	number;
		mf_func		func;
		mf_app		app;
		mf_tuple	tuple;		
	};
};

struct mf_app_struct
{
	mf_value func;
	mf_value arg;
};

mf_value make_number(long int number);
mf_value make_func(mf_func func);
mf_value make_app(mf_value func, mf_value arg);
mf_value make_tuple(int length);

void init(int size);
void push(mf_value value);

void error(const char* message);
void reduce(mf_value value);



#endif // _MF_RUNTIME_H