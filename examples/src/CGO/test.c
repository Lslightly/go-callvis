#include "test.h"
extern void C2GO();
void test() {
	printf("c test func\n");
	fflush(stdout);
	#ifdef A
	C2GO();
	#else
	;
	#endif
	#ifdef B
	C2GO();
	#endif
}
void GO2C() {
	printf("GO2C reaches C side\n");
	fflush(stdout);
}