#pragma once
#include <windows.h>
#include <stdio.h>
#include <netfw.h>

HRESULT __stdcall __declspec(dllexport) CreateRule(WCHAR* name, NET_FW_ACTION action, NET_FW_RULE_DIRECTION direction, LONG protocol, WCHAR* localAddresses, WCHAR* localPorts, WCHAR* remoteAddresses, WCHAR* remotePorts);
HRESULT __stdcall __declspec(dllexport) DeleteRule(WCHAR* name);
DWORD __stdcall __declspec(dllexport) RuleExists(WCHAR* name);

HRESULT initializeFirewallPolicy(INetFwPolicy2** ppNetFwPolicy2);
void cleanup(INetFwPolicy2* pNetFwPolicy2, INetFwRules* pNetFwRules, INetFwRule* pNetFwRule);
boolean portAllowed(LONG p);
void cleanupBSTR(BSTR n, BSTR lports, BSTR laddrs, BSTR rports, BSTR raddrs);
HRESULT initializeFirewallRule(INetFwRule** ppNetFwRule);
DWORD checkRule(INetFwRules *pRules, INetFwRule **ppRule, BSTR n);
