
// || Constants.

const int motion_pin = A5;
const int motion_umbral = 4;

const int current_pin = A0;

// || Program logic.

double motion_avg = 0;
double motion_navg = 0;

double motion_scan() {
  double val = analogRead(motion_pin);
  motion_navg++;
  motion_avg = motion_avg * (motion_navg - 1) / motion_navg + (val / motion_navg);
  return val;
}

bool motion_hasMotion(double scanValue) {
  return abs(scanValue - motion_avg) > motion_umbral;
}

double current_value() {
  int val = analogRead(current_pin);
  float volts = (val * 5.0) / 1024.0;
  return abs((volts - 2.5) / 0.1);
}

void setup() {
  Serial.begin(9600);
}

void loop() {
  double scan_value;
  if (millis() % 100 == 0) {
    scan_value = motion_scan();
  }

  if (millis() % 300 == 0) {
    Serial.print("# Val: ");
    Serial.print(scan_value);
    Serial.print(", Avg: ");
    Serial.println(motion_avg);

    Serial.print(motion_hasMotion(scan_value) ? "Y" : "N");
    Serial.print(" ");
    Serial.println(current_value());
  }

  if (millis() % 10000 == 0) {
    motion_avg = 0;
    motion_navg = 0;
  }
}
