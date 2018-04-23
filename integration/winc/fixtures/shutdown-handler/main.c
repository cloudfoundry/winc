#include <stdio.h>
#include <windows.h>
#include <Processthreadsapi.h>

BOOL WINAPI HandlerRoutine(_In_ DWORD dwCtrlType) {
  /* if (dwCtrlType != CTRL_SHUTDOWN_EVENT) { */
  /*   return TRUE; */
  /* } */

  printf("event received %d\n",dwCtrlType);
  exit(1);
  return TRUE;
}

int main() {
  if (SetConsoleCtrlHandler(HandlerRoutine, TRUE) == FALSE) { exit(1); }
  while (TRUE) { Sleep(1000); }
}

