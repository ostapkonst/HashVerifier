// Delivers kAEOpenDocuments (Finder double-click, "Open With", Dock drops) to Go.
// Registered once from Install(); the process keeps the singleton handler alive on purpose.

#import <Cocoa/Cocoa.h>

extern void goOpenFile(char *path, int count);

@interface HVOpenDocumentsHandler : NSObject
- (void)handleOpenDocumentsEvent:(NSAppleEventDescriptor *)event
                  withReplyEvent:(NSAppleEventDescriptor *)replyEvent;
@end

@implementation HVOpenDocumentsHandler

- (void)handleOpenDocumentsEvent:(NSAppleEventDescriptor *)event
                  withReplyEvent:(NSAppleEventDescriptor *)replyEvent {
    (void)replyEvent;

    NSAppleEventDescriptor *list = [event paramDescriptorForKeyword:keyDirectObject];
    if (!list) {
        return;
    }

    // The direct object is a list of files, but a lone descriptor is legal too.
    NSInteger count = 1;
    NSAppleEventDescriptor *firstItem = list;
    if ([list descriptorType] == typeAEList) {
        count = [list numberOfItems];
        if (count < 1) {
            return;
        }
        firstItem = [list descriptorAtIndex:1];
    }

    @autoreleasepool {
        NSAppleEventDescriptor *urlDesc = [firstItem coerceToDescriptorType:typeFileURL];
        NSString *urlString = urlDesc ? [urlDesc stringValue] : [firstItem stringValue];
        NSURL *url = urlString ? [NSURL URLWithString:urlString] : nil;
        const char *path = url ? [url fileSystemRepresentation] : NULL;
        if (path) {
            goOpenFile((char *)path, (int)count);
        }
    }
}

@end

void installOpenDocumentsHandler(void) {
    static HVOpenDocumentsHandler *handler = nil;
    if (handler) {
        return;
    }
    handler = [[HVOpenDocumentsHandler alloc] init];
    [[NSAppleEventManager sharedAppleEventManager]
        setEventHandler:handler
            andSelector:@selector(handleOpenDocumentsEvent:withReplyEvent:)
          forEventClass:kCoreEventClass
             andEventID:kAEOpenDocuments];
}
