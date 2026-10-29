#ifndef HBD_ABE_BOOTSTRAP_H
#define HBD_ABE_BOOTSTRAP_H

#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include "bootstrap_layout.h"

#ifdef __cplusplus
extern "C" {
#endif

__declspec(dllexport) ULONG_PTR WINAPI Bootstrap(LPVOID lpParameter);

#ifdef __cplusplus
}
#endif

#endif // HBD_ABE_BOOTSTRAP_H
