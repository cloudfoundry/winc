#pragma once
#include <windows.h>
#include <stdio.h>
#include <fsrmquota.h>

#if !defined(FSRM_E_NOT_FOUND)
// taken from: https://msdn.microsoft.com/en-us/library/dd350291.aspx
#define FSRM_E_NOT_FOUND 0x80045301
#endif

HRESULT __stdcall __declspec(dllexport) SetQuota(WCHAR* volume, ULONGLONG limit);
HRESULT __stdcall __declspec(dllexport) GetQuotaUsed(WCHAR* volume, PULONGLONG quotaUsed);
HRESULT initializeQuotaManager(IFsrmQuotaManager** pqmOut);
void cleanup(IFsrmQuotaManager* pqm, IFsrmQuota* pQuota);
