using System.Runtime.InteropServices;

namespace Misetanibox.Lite;

// Native notification-area icon: double click to show, right click to exit.
internal sealed class TrayIcon : IDisposable
{
    private const uint Callback = 0x8001;
    private readonly nint hwnd;
    private readonly WindowProc callback;
    private readonly nint previous;
    private readonly Action show, exit;
    private NotifyData data;
    public bool Ready { get; }
    public TrayIcon(nint window, Action show, Action exit)
    {
        hwnd = window; this.show = show; this.exit = exit; callback = Handle;
        previous = SetWindowLongPtr(hwnd, -4, Marshal.GetFunctionPointerForDelegate(callback));
        data = new NotifyData { Size = (uint)Marshal.SizeOf<NotifyData>(), Window = hwnd, Id = 1, Flags = 7, Message = Callback, Icon = LoadIcon(0, (nint)32512), Tip = "Misetanibox Lite · двойной щелчок: открыть; правая кнопка: выйти", Info = "", InfoTitle = "" };
        Ready = previous != 0 && ShellNotifyIcon(0, ref data);
    }
    private nint Handle(nint window, uint message, nuint w, nint l)
    {
        if (message == Callback) { if ((uint)l == 0x203) show(); if ((uint)l == 0x205) exit(); return 0; }
        return CallWindowProc(previous, window, message, w, l);
    }
    public void Dispose() { ShellNotifyIcon(2, ref data); if (previous != 0) SetWindowLongPtr(hwnd, -4, previous); }
    [UnmanagedFunctionPointer(CallingConvention.Winapi)] private delegate nint WindowProc(nint h, uint m, nuint w, nint l);
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)] private struct NotifyData
    {
        public uint Size; public nint Window; public uint Id, Flags, Message; public nint Icon;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 128)] public string Tip;
        public uint State, StateMask;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 256)] public string Info;
        public uint Timeout;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 64)] public string InfoTitle;
        public uint InfoFlags; public Guid Guid; public nint Balloon;
    }
    [DllImport("shell32.dll", EntryPoint = "Shell_NotifyIconW", CharSet = CharSet.Unicode)] [return: MarshalAs(UnmanagedType.Bool)] private static extern bool ShellNotifyIcon(uint message, ref NotifyData data);
    [DllImport("user32.dll", EntryPoint = "SetWindowLongPtrW")] private static extern nint SetWindowLongPtr(nint h, int index, nint value);
    [DllImport("user32.dll", EntryPoint = "CallWindowProcW")] private static extern nint CallWindowProc(nint old, nint h, uint message, nuint w, nint l);
    [DllImport("user32.dll", EntryPoint = "LoadIconW")] private static extern nint LoadIcon(nint instance, nint name);
}
