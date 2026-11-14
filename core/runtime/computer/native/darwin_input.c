// Native CoreGraphics input for Danmo Work's computer tool.
// Built at packaging time; no compiler is required on the user's machine.
#include <ApplicationServices/ApplicationServices.h>
#include <Carbon/Carbon.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <ctype.h>

static int fail(const char *message) {
    fprintf(stderr, "%s\n", message);
    return 1;
}

static CGEventFlags flags_for(const char *modifiers) {
    CGEventFlags flags = 0;
    if (modifiers == NULL) return flags;
    if (strstr(modifiers, "shift") != NULL) flags |= kCGEventFlagMaskShift;
    if (strstr(modifiers, "ctrl") != NULL) flags |= kCGEventFlagMaskControl;
    if (strstr(modifiers, "alt") != NULL) flags |= kCGEventFlagMaskAlternate;
    if (strstr(modifiers, "meta") != NULL) flags |= kCGEventFlagMaskCommand;
    return flags;
}

static int post(CGEventRef event, CGEventFlags flags) {
    if (event == NULL) return fail("could not create CoreGraphics event");
    CGEventSetFlags(event, flags);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
    return 0;
}

static int mouse(CGEventType type, CGMouseButton button, int x, int y, int clicks, const char *modifiers) {
    CGEventRef event = CGEventCreateMouseEvent(NULL, type, CGPointMake(x, y), button);
    if (event == NULL) return fail("could not create mouse event");
    CGEventSetIntegerValueField(event, kCGMouseEventClickState, clicks);
    return post(event, flags_for(modifiers));
}

static int displays(void) {
    uint32_t count = 0;
    if (CGGetActiveDisplayList(0, NULL, &count) != kCGErrorSuccess || count == 0) return fail("no active displays");
    CGDirectDisplayID *ids = calloc(count, sizeof(CGDirectDisplayID));
    if (ids == NULL) return fail("out of memory");
    if (CGGetActiveDisplayList(count, ids, &count) != kCGErrorSuccess) { free(ids); return fail("cannot enumerate displays"); }
    for (uint32_t i = 0; i < count; i++) {
        CGRect r = CGDisplayBounds(ids[i]);
        printf("%u\t%d\t%d\t%d\t%d\n", (unsigned)ids[i], (int)r.origin.x, (int)r.origin.y, (int)r.size.width, (int)r.size.height);
    }
    free(ids);
    return 0;
}

static int windows(void) {
    CFArrayRef list = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (list == NULL) return fail("cannot enumerate windows; grant Screen Recording permission");
    CFIndex count = CFArrayGetCount(list);
    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef item = CFArrayGetValueAtIndex(list, i);
        CFNumberRef number = CFDictionaryGetValue(item, kCGWindowNumber);
        CFNumberRef layer = CFDictionaryGetValue(item, kCGWindowLayer);
        CFNumberRef pid = CFDictionaryGetValue(item, kCGWindowOwnerPID);
        CFStringRef owner = CFDictionaryGetValue(item, kCGWindowOwnerName);
        CFStringRef title = CFDictionaryGetValue(item, kCGWindowName);
        CFDictionaryRef bounds = CFDictionaryGetValue(item, kCGWindowBounds);
        int id = 0, level = 0, process = 0;
        if (number) CFNumberGetValue(number, kCFNumberIntType, &id);
        if (layer) CFNumberGetValue(layer, kCFNumberIntType, &level);
        if (pid) CFNumberGetValue(pid, kCFNumberIntType, &process);
        CGRect rect;
        if (level != 0 || id == 0 || bounds == NULL || !CGRectMakeWithDictionaryRepresentation(bounds, &rect)) continue;
        char app[512] = "", name[2048] = "";
        if (owner) CFStringGetCString(owner, app, sizeof(app), kCFStringEncodingUTF8);
        if (title) CFStringGetCString(title, name, sizeof(name), kCFStringEncodingUTF8);
        for (char *p = app; *p; p++) if (*p == '\t' || *p == '\n' || *p == '\r') *p = ' ';
        for (char *p = name; *p; p++) if (*p == '\t' || *p == '\n' || *p == '\r') *p = ' ';
        printf("%d\t%d\t%s\t%d\t%d\t%d\t%d\t%s\n", id, process, app, (int)rect.origin.x, (int)rect.origin.y, (int)rect.size.width, (int)rect.size.height, name);
    }
    CFRelease(list);
    return 0;
}

static CGKeyCode ascii_keycode(char ascii) {
    TISInputSourceRef source = TISCopyCurrentKeyboardLayoutInputSource();
    if (source == NULL) return UINT16_MAX;
    CFDataRef data = TISGetInputSourceProperty(source, kTISPropertyUnicodeKeyLayoutData);
    if (data == NULL) { CFRelease(source); return UINT16_MAX; }
    const UCKeyboardLayout *layout = (const UCKeyboardLayout *)CFDataGetBytePtr(data);
    UniChar chars[4];
    UniCharCount actual = 0;
    for (CGKeyCode code = 0; code < 128; code++) {
        UInt32 dead = 0;
        if (UCKeyTranslate(layout, code, kUCKeyActionDisplay, 0, LMGetKbdType(), kUCKeyTranslateNoDeadKeysMask, &dead, 4, &actual, chars) == noErr && actual == 1 && chars[0] == (UniChar)ascii) {
            CFRelease(source);
            return code;
        }
    }
    CFRelease(source);
    return UINT16_MAX;
}

static int key_event(const char *text, int hold_seconds) {
    const char *modifiers = NULL;
    const char *separator = strchr(text, '\t');
    if (separator != NULL) {
        modifiers = separator + 1;
    }
    CGKeyCode code = UINT16_MAX;
    struct { const char *name; CGKeyCode code; } special[] = {
        {"return",36},{"enter",36},{"tab",48},{"space",49},{"escape",53},{"esc",53},
        {"delete",51},{"backspace",51},{"up",126},{"down",125},{"left",123},{"right",124},
        {"home",115},{"end",119},{"pageup",116},{"pagedown",121},
        {"f1",122},{"f2",120},{"f3",99},{"f4",118},{"f5",96},{"f6",97},{"f7",98},
        {"f8",100},{"f9",101},{"f10",109},{"f11",103},{"f12",111}
    };
    char name[32] = {0};
    size_t len = separator ? (size_t)(separator - text) : strlen(text);
    if (len >= sizeof(name)) return fail("key name is too long");
    for (size_t i = 0; i < len; i++) name[i] = (char)tolower((unsigned char)text[i]);
    for (size_t i = 0; i < sizeof(special)/sizeof(special[0]); i++) {
        if (strcmp(name, special[i].name) == 0) { code = special[i].code; break; }
    }
    if (code == UINT16_MAX && len == 1) code = ascii_keycode(name[0]);
    if (code == UINT16_MAX) return fail("unsupported key");
    CGEventFlags flags = flags_for(modifiers);
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, code, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, code, false);
    if (!down || !up) return fail("could not create key events");
    CGEventSetFlags(down, flags);
    CGEventSetFlags(up, flags);
    CGEventPost(kCGHIDEventTap, down);
    if (hold_seconds > 0) sleep((unsigned int)hold_seconds);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
    return 0;
}

static int type_unicode(const char *text) {
    CFStringRef string = CFStringCreateWithCString(NULL, text, kCFStringEncodingUTF8);
    if (string == NULL) return fail("text is not valid UTF-8");
    CFIndex length = CFStringGetLength(string);
    UniChar *buffer = calloc((size_t)length + 1, sizeof(UniChar));
    if (buffer == NULL) { CFRelease(string); return fail("out of memory"); }
    CFStringGetCharacters(string, CFRangeMake(0, length), buffer);
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, 0, false);
    int status = 0;
    if (down == NULL || up == NULL) {
        status = fail("could not create keyboard event");
    } else {
        CGEventKeyboardSetUnicodeString(down, (UniCharCount)length, buffer);
        CGEventKeyboardSetUnicodeString(up, (UniCharCount)length, buffer);
        CGEventPost(kCGHIDEventTap, down);
        CGEventPost(kCGHIDEventTap, up);
    }
    if (down) CFRelease(down);
    if (up) CFRelease(up);
    free(buffer);
    CFRelease(string);
    return status;
}

int main(int argc, char **argv) {
    if (argc < 2) return fail("usage: dq-darwin-input <probe|displays|windows|move|click|drag|button|scroll|type|cursor> ...");
    const char *cmd = argv[1];
    if (strcmp(cmd, "probe") == 0) {
        printf("screen=%d accessibility=%d\n", CGPreflightScreenCaptureAccess() ? 1 : 0, AXIsProcessTrusted() ? 1 : 0);
        return 0;
    }
    if (strcmp(cmd, "displays") == 0) return displays();
    if (strcmp(cmd, "windows") == 0) return windows();
    if (strcmp(cmd, "cursor") == 0) {
        CGEventRef event = CGEventCreate(NULL);
        if (!event) return fail("cannot read cursor position");
        CGPoint point = CGEventGetLocation(event);
        printf("%d,%d\n", (int)point.x, (int)point.y);
        CFRelease(event);
        return 0;
    }
    if (strcmp(cmd, "type") == 0 && argc == 3) return type_unicode(argv[2]);
    if (strcmp(cmd, "key") == 0 && (argc == 3 || argc == 4)) return key_event(argv[2], argc == 4 ? atoi(argv[3]) : 0);
    if (strcmp(cmd, "move") == 0 && argc >= 4) {
        return mouse(kCGEventMouseMoved, kCGMouseButtonLeft, atoi(argv[2]), atoi(argv[3]), 0, argc > 4 ? argv[4] : NULL);
    }
    if (strcmp(cmd, "click") == 0 && argc >= 6) {
        int x = atoi(argv[3]), y = atoi(argv[4]), clicks = atoi(argv[5]);
        const char *mods = argc > 6 ? argv[6] : NULL;
        CGEventType down = kCGEventLeftMouseDown, up = kCGEventLeftMouseUp;
        CGMouseButton button = kCGMouseButtonLeft;
        if (strcmp(argv[2], "right") == 0) { down = kCGEventRightMouseDown; up = kCGEventRightMouseUp; button = kCGMouseButtonRight; }
        if (strcmp(argv[2], "middle") == 0) { down = kCGEventOtherMouseDown; up = kCGEventOtherMouseUp; button = kCGMouseButtonCenter; }
        for (int i = 1; i <= clicks; i++) {
            if (mouse(down, button, x, y, i, mods) || mouse(up, button, x, y, i, mods)) return 1;
            if (i < clicks) usleep(45000);
        }
        return 0;
    }
    if (strcmp(cmd, "button") == 0 && argc == 5) {
        int x = atoi(argv[3]), y = atoi(argv[4]);
        CGEventType type = strcmp(argv[2], "down") == 0 ? kCGEventLeftMouseDown : kCGEventLeftMouseUp;
        return mouse(type, kCGMouseButtonLeft, x, y, 1, NULL);
    }
    if (strcmp(cmd, "drag") == 0 && argc == 6) {
        int x0 = atoi(argv[2]), y0 = atoi(argv[3]), x1 = atoi(argv[4]), y1 = atoi(argv[5]);
        if (mouse(kCGEventLeftMouseDown, kCGMouseButtonLeft, x0, y0, 1, NULL)) return 1;
        for (int i = 1; i <= 12; i++) {
            if (mouse(kCGEventLeftMouseDragged, kCGMouseButtonLeft, x0 + (x1-x0)*i/12, y0 + (y1-y0)*i/12, 1, NULL)) return 1;
            usleep(12000);
        }
        return mouse(kCGEventLeftMouseUp, kCGMouseButtonLeft, x1, y1, 1, NULL);
    }
    if (strcmp(cmd, "scroll") == 0 && argc == 6) {
        if (mouse(kCGEventMouseMoved, kCGMouseButtonLeft, atoi(argv[2]), atoi(argv[3]), 0, NULL)) return 1;
        int amount = atoi(argv[5]);
        int vertical = 0, horizontal = 0;
        if (strcmp(argv[4], "up") == 0) vertical = amount;
        else if (strcmp(argv[4], "down") == 0) vertical = -amount;
        else if (strcmp(argv[4], "left") == 0) horizontal = -amount;
        else if (strcmp(argv[4], "right") == 0) horizontal = amount;
        else return fail("unknown scroll direction");
        CGEventRef event = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitLine, 2, vertical, horizontal);
        return post(event, 0);
    }
    return fail("invalid command or arguments");
}