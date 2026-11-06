import AVFoundation
import Foundation

// Usage: avtrigger <camera|mic> <seconds>
// Starts a real capture session (with an output that pulls samples) so the
// device is actually streaming, then exits after <seconds>.

let mode = CommandLine.arguments[1]
let seconds = Double(CommandLine.arguments[2])!

final class Sink: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate,
    AVCaptureAudioDataOutputSampleBufferDelegate {
    func captureOutput(_ output: AVCaptureOutput,
                       didOutput sampleBuffer: CMSampleBuffer,
                       from connection: AVCaptureConnection) {}
}

let sink = Sink()
let queue = DispatchQueue(label: "avtrigger")

let session = AVCaptureSession()
session.beginConfiguration()

switch mode {
case "camera":
    guard let device = AVCaptureDevice.default(for: .video) else {
        FileHandle.standardError.write("no video device\n".data(using: .utf8)!)
        exit(1)
    }
    session.addInput(try AVCaptureDeviceInput(device: device))
    let out = AVCaptureVideoDataOutput()
    out.setSampleBufferDelegate(sink, queue: queue)
    session.addOutput(out)

case "mic":
    guard let device = AVCaptureDevice.default(for: .audio) else {
        FileHandle.standardError.write("no audio device\n".data(using: .utf8)!)
        exit(1)
    }
    session.addInput(try AVCaptureDeviceInput(device: device))
    let out = AVCaptureAudioDataOutput()
    out.setSampleBufferDelegate(sink, queue: queue)
    session.addOutput(out)

default:
    exit(2)
}

session.commitConfiguration()
session.startRunning()
guard session.isRunning else {
    FileHandle.standardError.write("session failed to start\n".data(using: .utf8)!)
    exit(1)
}
print("\(mode) streaming")
fflush(stdout)
Thread.sleep(forTimeInterval: seconds)
session.stopRunning()
