#pragma once
#include <windows.h>
#include <stdio.h>
#include <fsrmquota.h>

HRESULT __stdcall __declspec(dllexport) SetQuota(WCHAR* volume, ULONGLONG limit);
void cleanup(IFsrmQuotaManager* pqm, IFsrmQuota* pQuota);
