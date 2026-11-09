// Delivers open-document requests (Finder double-click, "Open With", Dock drops)
// through an NSApplication delegate. GDK never calls finishLaunching, so the app
// would otherwise stay half-launched and AppKit would not invoke these callbacks.

#import <Cocoa/Cocoa.h>

extern void goOpenFile(char *path, int count);

@interface HVAppDelegate : NSObject <NSApplicationDelegate>
@end

@implementation HVAppDelegate

- (BOOL)application:(NSApplication *)app openFile:(NSString *)filename {
    (void)app;
    if (filename) {
        goOpenFile((char *)[filename fileSystemRepresentation], 1);
    }
    return YES;
}

- (void)application:(NSApplication *)app openFiles:(NSArray<NSString *> *)filenames {
    NSUInteger count = [filenames count];
    if (count > 0) {
        goOpenFile((char *)[[filenames objectAtIndex:0] fileSystemRepresentation], (int)count);
    }
    [app replyToOpenOrPrint:NSApplicationDelegateReplySuccess];
}

@end

void installOpenDocumentsHandler(void) {
    static BOOL installed = NO;
    if (installed) {
        return;
    }
    installed = YES;
    [NSApp setDelegate:[[HVAppDelegate alloc] init]];
    [NSApp finishLaunching];
}
