#ifndef UNICODE
#define UNICODE
#endif

#ifndef CINTERFACE
#define CINTERFACE
#endif

#include <windows.h>
#include <stdio.h>
#include <netfw.h>  // firewall objects
#include "firewall.h"

HRESULT __stdcall  CreateRule(WCHAR* name, NET_FW_ACTION action, NET_FW_RULE_DIRECTION direction, LONG protocol, WCHAR* localAddresses, WCHAR* localPorts, WCHAR* remoteAddresses, WCHAR* remotePorts) {
  HRESULT hr = S_OK;
  INetFwPolicy2 *pPolicy2 = NULL;
  INetFwRules *pRules = NULL;
  INetFwRule *pRule = NULL;


  hr = initializeFirewallPolicy(&pPolicy2);
  if (FAILED(hr)) {
    cleanup(pPolicy2, pRules, pRule);
    return hr;
  }

  hr = pPolicy2->lpVtbl->get_Rules(pPolicy2, &pRules);
  if (FAILED(hr)) {
    wprintf(L"pPolicy2->get_Rules failed 0x%x\n", hr);
    cleanup(pPolicy2, pRules, pRule);
    return hr;
  }

  hr = initializeFirewallRule(&pRule);
  if (FAILED(hr)) {
    wprintf(L"failed creating INetFwRule: 0x%x.\n", hr);
    cleanup(pPolicy2, pRules, pRule);
    return hr;
  }

  if (!name) {
    wprintf(L"must provide name for firewall rule\n");
    cleanup(pPolicy2, pRules, pRule);
    return 1;
  }

  BSTR n = SysAllocString(name);
  pRule->lpVtbl->put_Name(pRule, n);
  pRule->lpVtbl->put_Direction(pRule, direction);
  pRule->lpVtbl->put_Action(pRule, action);
  pRule->lpVtbl->put_Enabled(pRule, VARIANT_TRUE);

  if (protocol != 0) {
    pRule->lpVtbl->put_Protocol(pRule, protocol);
  }

  BSTR lports = NULL;
  BSTR laddrs = NULL;
  BSTR rports = NULL;
  BSTR raddrs = NULL;

  if (localAddresses) {
    laddrs = SysAllocString(localAddresses);
    pRule->lpVtbl->put_LocalAddresses(pRule, laddrs);
  }

  if (localPorts && portAllowed(protocol)) {
    lports = SysAllocString(localPorts);
    pRule->lpVtbl->put_LocalPorts(pRule, lports);
  }

  if (remoteAddresses) {
    raddrs = SysAllocString(remoteAddresses);
    pRule->lpVtbl->put_RemoteAddresses(pRule, raddrs);
  }

  if (remotePorts && portAllowed(protocol)) {
    rports = SysAllocString(remotePorts);
    pRule->lpVtbl->put_RemotePorts(pRule, rports);
  }

  hr = pRules->lpVtbl->Add(pRules, pRule);
  cleanupBSTR(n, lports, laddrs, rports, raddrs);
  cleanup(pPolicy2, pRules, pRule);

  if (FAILED(hr)) {
    wprintf(L"pPolicy2->get_Rules failed: 0x%x\n", hr);
    return hr;
  }

  return S_OK;
}

boolean portAllowed(LONG p) {
  return (p == NET_FW_IP_PROTOCOL_TCP) || (p == NET_FW_IP_PROTOCOL_UDP);
}

void cleanupBSTR(BSTR n, BSTR lports, BSTR laddrs, BSTR rports, BSTR raddrs) {
  SysFreeString(n);
  SysFreeString(lports);
  SysFreeString(laddrs);
  SysFreeString(rports);
  SysFreeString(raddrs);
}

HRESULT __stdcall  DeleteRule(WCHAR* name) {
  HRESULT hr = S_OK;
  INetFwPolicy2 *pPolicy2 = NULL;
  INetFwRules *pRules = NULL;
  INetFwRule *pRule = NULL;


  hr = initializeFirewallPolicy(&pPolicy2);
  if (FAILED(hr)) {
    cleanup(pPolicy2, pRules, pRule);
    return hr;
  }

  hr = pPolicy2->lpVtbl->get_Rules(pPolicy2, &pRules);
  if (FAILED(hr)) {
    wprintf(L"pPolicy2->get_Rules failed: 0x%x\n", hr);
    cleanup(pPolicy2, pRules, pRule);
    return hr;
  }

  DWORD exists = 0;
  HRESULT ret = S_OK;

  BSTR n = SysAllocString(name);

  // multiple rules can have the same name, and rules->Remove will
  // only remove 1 of them. So keep checking until they're all
  // gone
  while (exists = checkRule(pRules, &pRule, n)) {
    hr = pRules->lpVtbl->Remove(pRules, n);
    if (FAILED(hr)) {
      wprintf(L"pPolicy2->Remove(%s) failed: 0x%x\n", name, hr);
      ret = hr;
    }
  }

  SysFreeString(n);
  cleanup(pPolicy2, pRules, pRule);

  if (exists == -1) {
    return 1;
  }

  return ret;
}

// returns 1 if the rule exists, 0 if the rule doesn't, and -1 on an error
DWORD __stdcall  RuleExists(WCHAR* name) {
  HRESULT hr = S_OK;
  DWORD ret = 0;
  INetFwPolicy2 *pPolicy2 = NULL;
  INetFwRules *pRules = NULL;
  INetFwRule *pRule = NULL;


  hr = initializeFirewallPolicy(&pPolicy2);
  if (FAILED(hr)) {
    cleanup(pPolicy2, pRules, pRule);
    return -1;
  }

  hr = pPolicy2->lpVtbl->get_Rules(pPolicy2, &pRules);
  if (FAILED(hr)) {
    wprintf(L"pPolicy2->get_Rules failed: 0x%x\n", hr);
    cleanup(pPolicy2, pRules, pRule);
    return -1;
  }

  BSTR n = SysAllocString(name);
  ret = checkRule(pRules, &pRule, n);
  SysFreeString(n);
  cleanup(pPolicy2, pRules, pRule);

  return ret;
}

DWORD checkRule(INetFwRules *pRules, INetFwRule **ppRule, BSTR n) {
  HRESULT hr = S_OK;

  hr = pRules->lpVtbl->Item(pRules, n, ppRule);

  if (FAILED(hr)) {
    if (hr == HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND)) {
      return 0;
    } else {
      wprintf(L"pPolicy2->Item(%s) failed: 0x%x\n", n, hr);
      return -1;
    }
  }

  return 1;
}


HRESULT initializeFirewallPolicy(INetFwPolicy2** ppNetFwPolicy2) {
  HRESULT hr = S_OK;
  IID CLSID_NetFwPolicy2;
  IID IID_INetFwPolicy2;

  hr = IIDFromString(OLESTR("{E2B3C97F-6AE1-41AC-817A-F6F92166D7DD}"), &CLSID_NetFwPolicy2);
  if (FAILED(hr))
  {
    wprintf(L"IIDFromString(CLSID_NetFwPolicy2) failed, 0x%x.\n", hr);
    return hr;
  }

  hr = IIDFromString(OLESTR("{98325047-C671-4174-8D81-DEFCD3F03186}"), &IID_INetFwPolicy2);
  if (FAILED(hr))
  {
    wprintf(L"IIDFromString(IID_INetFwPolicy2) failed, 0x%x.\n", hr);
    return hr;
  }

  hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
  if (FAILED(hr))
  {
    wprintf(L"CoInitializeEx() failed, 0x%x.\n", hr);
    return hr;
  }

  hr = CoCreateInstance(
    &CLSID_NetFwPolicy2,
    NULL,
    CLSCTX_INPROC_SERVER,
    &IID_INetFwPolicy2,
    (void**)(ppNetFwPolicy2));

  if (FAILED(hr))
  {
    wprintf(L"CoCreateInstance(INetFwPolicy2) failed, 0x%x.\n", hr);
    return hr;
  }

  return S_OK;
}

HRESULT initializeFirewallRule(INetFwRule** ppNetFwRule) {
  HRESULT hr = S_OK;
  IID CLSID_NetFwRule;
  IID IID_INetFwRule;

  hr = IIDFromString(OLESTR("{2C5BC43E-3369-4C33-AB0C-BE9469677AF4}"), &CLSID_NetFwRule);
  if (FAILED(hr))
  {
    wprintf(L"IIDFromString(CLSID_NetFwRule) failed, 0x%x.\n", hr);
    return hr;
  }

  hr = IIDFromString(OLESTR("{AF230D27-BABA-4E42-ACED-F524F22CFCE2}"), &IID_INetFwRule);
  if (FAILED(hr))
  {
    wprintf(L"IIDFromString(IID_INetFwRule) failed, 0x%x.\n", hr);
    return hr;
  }

  hr = CoCreateInstance(
    &CLSID_NetFwRule,
    NULL,
    CLSCTX_INPROC_SERVER,
    &IID_INetFwRule,
    (void**)(ppNetFwRule));

  if (FAILED(hr))
  {
    wprintf(L"CoCreateInstance(NetFwRule) failed, 0x%x.\n", hr);
    return hr;
  }

  return S_OK;
}

void cleanup(INetFwPolicy2* pNetFwPolicy2, INetFwRules* pNetFwRules, INetFwRule* pNetFwRule) {
  if (pNetFwPolicy2)
    pNetFwPolicy2->lpVtbl->Release(pNetFwPolicy2);

  if (pNetFwRules)
    pNetFwRules->lpVtbl->Release(pNetFwRules);

  if (pNetFwRule)
    pNetFwRule->lpVtbl->Release(pNetFwRule);

  CoUninitialize();
}

