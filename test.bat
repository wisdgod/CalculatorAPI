@echo off
REM ����һ�����ڲ��Ժ�����GoӦ�ó�����������ļ�

cd /d "%~dp0"

set CGO_ENABLED=1

REM ����Go����
echo �������в���...
go run .

set CGO_ENABLED=

REM ��ͣ�Ա���������ʾ�����ڴ�
REM pause

REM ������Goģ�黺�棺
REM Ҫ���Goģ�黺�棬����ʹ���������
REM go clean -modcache
REM go clean -cache -modcache -i -r